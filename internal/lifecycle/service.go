package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloudmatrix/internal/adb"
	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/provider"
	"cloudmatrix/internal/store"
)

// Service 实例生命周期管理服务接口
type Service interface {
	Start(ctx context.Context, instanceID string) (*domain.Instance, error)
	Stop(ctx context.Context, instanceID string) (*domain.Instance, error)
	Restart(ctx context.Context, instanceID string) (*domain.Instance, error)
	GetState(ctx context.Context, instanceID string) (domain.InstanceState, error)
}

// LifecycleService 实例生命周期核心服务实现
type LifecycleService struct {
	store        store.Store
	provider     provider.Provider
	adbClient    readinessChecker
	admission    *AdmissionController
	readyTimeout time.Duration
	stopTimeout  time.Duration

	// instanceLocks 保证对单个实例写操作的绝对串行
	instanceLocks sync.Map
	// activeStarts 跟踪正在启动中的实例任务，保证幂等合并
	activeStarts sync.Map
}

// startFuture 活跃启动任务结果句柄
type startFuture struct {
	done chan struct{}
	inst *domain.Instance
	err  error
}

type readinessChecker interface {
	CheckReadiness(context.Context, string) (*adb.ReadinessResult, error)
}

// NewService 创建生命周期服务
func NewService(
	store store.Store,
	provider provider.Provider,
	adbClient readinessChecker,
	admission *AdmissionController,
	readyTimeout time.Duration,
	stopTimeout time.Duration,
) *LifecycleService {
	return &LifecycleService{
		store:        store,
		provider:     provider,
		adbClient:    adbClient,
		admission:    admission,
		readyTimeout: readyTimeout,
		stopTimeout:  stopTimeout,
	}
}

// getLock 获取指定实例的互斥锁
func (s *LifecycleService) getLock(instanceID string) *sync.Mutex {
	lock, _ := s.instanceLocks.LoadOrStore(instanceID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// Start 幂等启动指定实例，带并发控制与就绪探测
func (s *LifecycleService) Start(ctx context.Context, instanceID string) (*domain.Instance, error) {
	// 0. 上下文预先检查
	if err := ctx.Err(); err != nil {
		return nil, domain.NewPlatformError(domain.ErrCodeCancelled, "启动请求已被取消", false)
	}

	// 1. 检查是否存在正在执行的活跃启动任务，存在则复用等待结果（幂等启动核心）
	if val, ok := s.activeStarts.Load(instanceID); ok {
		fut := val.(*startFuture)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-fut.done:
			return fut.inst, fut.err
		}
	}

	mu := s.getLock(instanceID)
	mu.Lock()

	// 二次检查活跃启动
	if val, ok := s.activeStarts.Load(instanceID); ok {
		mu.Unlock()
		fut := val.(*startFuture)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-fut.done:
			return fut.inst, fut.err
		}
	}

	// 注册新的启动 future
	fut := &startFuture{done: make(chan struct{})}
	s.activeStarts.Store(instanceID, fut)
	mu.Unlock()

	defer func() {
		s.activeStarts.Delete(instanceID)
		close(fut.done)
	}()

	inst, err := s.doStart(ctx, instanceID)
	fut.inst = inst
	fut.err = err
	return inst, err
}

func (s *LifecycleService) doStart(ctx context.Context, instanceID string) (*domain.Instance, error) {
	mu := s.getLock(instanceID)
	mu.Lock()
	defer mu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, domain.NewPlatformError(domain.ErrCodeCancelled, "启动请求已被取消", false)
	}

	// 1. 加载实例当前状态
	inst, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	if s.adbClient == nil {
		return nil, domain.NewPlatformError(domain.ErrCodeToolMissing, "ADB 就绪探测不可用", false)
	}
	current, err := s.currentInstance(ctx, inst.ProviderInstanceID)
	if err != nil {
		return nil, s.recordUnknown(ctx, inst, err)
	}
	inst.DesiredState = domain.StateReady
	switch current.ObservedState {
	case domain.StateStopped:
		if err := s.launchStopped(ctx, inst); err != nil {
			return nil, err
		}
	case domain.StateBooting, domain.StateReady:
		s.admission.ObserveRunning(inst.ID)
	default:
		return nil, s.recordUnknown(ctx, inst, fmt.Errorf("实例状态未确认，拒绝启动"))
	}

	if err := ctx.Err(); err != nil {
		return nil, s.recordUnknown(ctx, inst, err)
	}

	inst.AdbSerial = ""
	if err := s.saveObserved(ctx, inst, domain.StateBooting); err != nil {
		return nil, err
	}

	// Always use the serial reported after launch; the vendor may assign a new port.
	probeCtx, cancel := context.WithTimeout(ctx, s.readyTimeout)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for probeCtx.Err() == nil {
		discovered, discoverErr := s.provider.Discover(probeCtx)
		if discoverErr == nil {
			for _, current := range discovered {
				if current.ProviderInstanceID != inst.ProviderInstanceID || current.AdbSerial == "" ||
					(current.ObservedState != domain.StateBooting && current.ObservedState != domain.StateReady) {
					continue
				}
				res, probeErr := s.adbClient.CheckReadiness(probeCtx, current.AdbSerial)
				if probeErr == nil && res.Ready {
					inst.AdbSerial = current.AdbSerial
					if err := s.saveObserved(ctx, inst, domain.StateReady); err != nil {
						return nil, err
					}
					return inst, nil
				}
				break
			}
		}
		select {
		case <-probeCtx.Done():
		case <-ticker.C:
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, s.recordUnknown(ctx, inst, err)
	}
	return nil, s.recordUnknown(ctx, inst,
		domain.NewPlatformError(domain.ErrCodeBootTimeout, "等待实例 Android 启动就绪超时", false))
}

func (s *LifecycleService) currentInstance(ctx context.Context, providerID string) (*domain.Instance, error) {
	found, err := s.provider.Discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("核对厂商状态失败: %w", err)
	}
	for _, inst := range found {
		if inst.ProviderInstanceID == providerID {
			return inst, nil
		}
	}
	return nil, domain.NewPlatformError(domain.ErrCodeInstanceNotFound, "厂商未返回指定实例", false)
}

func (s *LifecycleService) launchStopped(ctx context.Context, inst *domain.Instance) error {
	if err := s.admission.AcquireRunningQuota(inst.ID); err != nil {
		return err
	}
	sent := false
	defer func() {
		if !sent {
			s.admission.ReleaseRunningQuota(inst.ID)
		}
	}()
	releaseSlot, err := s.admission.AcquireStartSlot(ctx)
	if err != nil {
		return err
	}
	defer releaseSlot()
	if err := s.saveObserved(ctx, inst, domain.StateStarting); err != nil {
		return err
	}
	sent = true
	if err := s.provider.Start(ctx, inst.ProviderInstanceID); err != nil {
		return s.recordUnknown(ctx, inst, fmt.Errorf("底层厂商启动结果未确认: %w", err))
	}
	return nil
}

// Stop 显式停止指定实例
func (s *LifecycleService) Stop(ctx context.Context, instanceID string) (*domain.Instance, error) {
	mu := s.getLock(instanceID)
	mu.Lock()
	defer mu.Unlock()

	inst, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	current, err := s.currentInstance(ctx, inst.ProviderInstanceID)
	if err != nil {
		return nil, s.recordUnknown(ctx, inst, err)
	}
	inst.DesiredState = domain.StateStopped
	switch current.ObservedState {
	case domain.StateStopped:
	case domain.StateBooting, domain.StateReady:
		if err := s.saveObserved(ctx, inst, domain.StateStopping); err != nil {
			return nil, err
		}
		if err := s.provider.Stop(ctx, inst.ProviderInstanceID); err != nil {
			return nil, s.recordUnknown(ctx, inst, fmt.Errorf("底层厂商停止结果未确认: %w", err))
		}
		confirmCtx, cancel := context.WithTimeout(ctx, s.stopTimeout)
		defer cancel()
		if err := s.confirmStopped(confirmCtx, inst.ProviderInstanceID); err != nil {
			return nil, s.recordUnknown(ctx, inst, fmt.Errorf("未能确认实例已停止: %w", err))
		}
	default:
		return nil, s.recordUnknown(ctx, inst, fmt.Errorf("实例状态未确认，拒绝停止"))
	}

	// 释放配额
	s.admission.ReleaseRunningQuota(instanceID)

	inst.AdbSerial = ""
	if err := s.saveObserved(ctx, inst, domain.StateStopped); err != nil {
		return nil, err
	}
	return inst, nil
}

func (s *LifecycleService) confirmStopped(ctx context.Context, providerID string) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		discovered, err := s.provider.Discover(ctx)
		if err == nil {
			for _, current := range discovered {
				if current.ProviderInstanceID == providerID && current.ObservedState == domain.StateStopped {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
	return ctx.Err()
}

// Restart 组合执行 Stop 与 Start
func (s *LifecycleService) Restart(ctx context.Context, instanceID string) (*domain.Instance, error) {
	_, err := s.Stop(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return s.Start(ctx, instanceID)
}

func (s *LifecycleService) saveObserved(ctx context.Context, inst *domain.Instance, state domain.InstanceState) error {
	inst.ObservedState = state
	if _, err := s.store.UpsertInstance(ctx, inst); err != nil {
		return fmt.Errorf("保存实例 %s 状态 %s: %w", inst.ID, state, err)
	}
	return nil
}

func (s *LifecycleService) recordUnknown(ctx context.Context, inst *domain.Instance, cause error) error {
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	inst.AdbSerial = ""
	return errors.Join(cause, s.saveObserved(saveCtx, inst, domain.StateUnknown))
}

// GetState 获取实例当前观测状态
func (s *LifecycleService) GetState(ctx context.Context, instanceID string) (domain.InstanceState, error) {
	inst, err := s.store.GetInstance(ctx, instanceID)
	if err != nil {
		return domain.StateUnknown, err
	}
	return inst.ObservedState, nil
}
