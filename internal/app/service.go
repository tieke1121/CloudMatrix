package app

import (
	"context"
	"fmt"
	"sync"

	"cloudmatrix/internal/adb"
	"cloudmatrix/internal/config"
	"cloudmatrix/internal/lifecycle"
	"cloudmatrix/internal/provider"
	"cloudmatrix/internal/store"
	"cloudmatrix/internal/stream"
	"cloudmatrix/internal/task"
)

// Service 桌面应用门面用例核心服务
type Service struct {
	cfg           *config.Config
	store         store.Store
	provider      provider.Provider
	adbClient     *adb.Client
	lifecycle     lifecycle.Service
	streamManager stream.Manager
	taskQueue     *task.Queue
	instanceMu    sync.RWMutex
	taskMu        sync.Mutex
	activeTasks   map[string]context.CancelFunc
	taskWorkers   sync.WaitGroup
	closing       bool
}

// NewService 创建桌面应用门面服务
func NewService(
	cfg *config.Config,
	st store.Store,
	prov provider.Provider,
	adbClient *adb.Client,
	lc lifecycle.Service,
	sm stream.Manager,
	tq *task.Queue,
) *Service {
	return &Service{
		cfg:           cfg,
		store:         st,
		provider:      prov,
		adbClient:     adbClient,
		lifecycle:     lc,
		streamManager: sm,
		taskQueue:     tq,
		activeTasks:   make(map[string]context.CancelFunc),
	}
}

// ListInstances 查询当前已纳管实例列表及其投屏状态
func (s *Service) ListInstances(ctx context.Context) ([]InstanceDTO, error) {
	instances, err := s.store.ListInstances(ctx)
	if err != nil {
		return nil, err
	}

	dtos := make([]InstanceDTO, 0, len(instances))
	for _, inst := range instances {
		hasStream := false
		if s.streamManager != nil {
			hasStream = s.streamManager.HasSession(inst.ID)
		}

		dtos = append(dtos, InstanceDTO{
			ID:                 inst.ID,
			HostID:             inst.HostID,
			ProviderType:       inst.ProviderType,
			ProviderInstanceID: inst.ProviderInstanceID,
			Name:               inst.Name,
			DesiredState:       inst.DesiredState,
			ObservedState:      inst.ObservedState,
			AdbSerial:          inst.AdbSerial,
			HasStream:          hasStream,
			CreatedAt:          inst.CreatedAt,
			UpdatedAt:          inst.UpdatedAt,
		})
	}

	return dtos, nil
}

// StartInstance 启动指定实例
func (s *Service) StartInstance(ctx context.Context, id string) (OperationResponse, error) {
	s.instanceMu.RLock()
	defer s.instanceMu.RUnlock()
	if s.lifecycle == nil {
		return OperationResponse{Success: false, Message: "生命周期服务未就绪"}, nil
	}
	if _, err := s.managedInstance(ctx, id); err != nil {
		return OperationResponse{}, err
	}

	_, err := s.lifecycle.Start(ctx, id)
	if err != nil {
		return OperationResponse{Success: false, Message: err.Error()}, err
	}

	return OperationResponse{Success: true, Message: "实例启动成功"}, nil
}

// StopInstance 停止指定实例
func (s *Service) StopInstance(ctx context.Context, id string) (OperationResponse, error) {
	s.instanceMu.RLock()
	defer s.instanceMu.RUnlock()
	if s.lifecycle == nil {
		return OperationResponse{Success: false, Message: "生命周期服务未就绪"}, nil
	}
	if _, err := s.managedInstance(ctx, id); err != nil {
		return OperationResponse{}, err
	}

	// 若处于投屏中，先关闭投屏会话
	if s.streamManager != nil && s.streamManager.HasSession(id) {
		if err := s.streamManager.StopSession(id); err != nil {
			return OperationResponse{}, fmt.Errorf("关闭投屏后再停止实例: %w", err)
		}
	}

	_, err := s.lifecycle.Stop(ctx, id)
	if err != nil {
		return OperationResponse{Success: false, Message: err.Error()}, err
	}

	return OperationResponse{Success: true, Message: "实例已停止"}, nil
}

// OpenStream 为指定实例打开独立投屏窗口
func (s *Service) OpenStream(ctx context.Context, id string) (OperationResponse, error) {
	s.instanceMu.RLock()
	defer s.instanceMu.RUnlock()
	return s.openStream(ctx, id)
}

func (s *Service) openStream(ctx context.Context, id string) (OperationResponse, error) {
	if err := ctx.Err(); err != nil {
		return OperationResponse{}, err
	}
	if s.streamManager == nil {
		return OperationResponse{Success: false, Message: "投屏管理器未就绪"}, nil
	}

	inst, err := s.managedInstance(ctx, id)
	if err != nil {
		return OperationResponse{Success: false, Message: err.Error()}, err
	}

	if err := s.checkCurrentSerial(ctx, inst); err != nil {
		return OperationResponse{}, err
	}

	opts := stream.ScrcpyOptions{
		Serial:      inst.AdbSerial,
		WindowTitle: fmt.Sprintf("CloudMatrix - %s (%s)", inst.Name, inst.AdbSerial),
		NoAudio:     true,
		StayAwake:   true,
		MaxFPS:      30,
	}

	_, startErr := s.streamManager.StartSession(ctx, id, opts)
	if startErr != nil {
		return OperationResponse{Success: false, Message: startErr.Error()}, startErr
	}

	return OperationResponse{Success: true, Message: "投屏会话已开启"}, nil
}

// CloseStream 关闭投屏窗口
func (s *Service) CloseStream(id string) (OperationResponse, error) {
	if s.streamManager == nil {
		return OperationResponse{Success: false, Message: "投屏管理器未就绪"}, nil
	}

	if err := s.streamManager.StopSession(id); err != nil {
		return OperationResponse{Success: false, Message: err.Error()}, err
	}

	return OperationResponse{Success: true, Message: "投屏已关闭"}, nil
}
