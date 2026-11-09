package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"cloudmatrix/internal/adb"
	"cloudmatrix/internal/app"
	"cloudmatrix/internal/config"
	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/lifecycle"
	"cloudmatrix/internal/provider/mumu"
	"cloudmatrix/internal/store"
	"cloudmatrix/internal/stream"
	"cloudmatrix/internal/task"
)

// App 应用核心结构体，绑定到 Wails 客户端
type App struct {
	ctx        context.Context
	cancel     context.CancelFunc
	store      store.Store
	dataLock   *os.File
	appService *app.Service
}

// NewApp 创建并装配 App 完整依赖
func NewApp(cfg *config.Config) (*App, error) {
	if err := cfg.Provider.VerifyProviderCLI(); err != nil {
		return nil, err
	}
	dataDir := cfg.DataDir
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}

	dbPath := filepath.Join(dataDir, "cloudmatrix.db")
	dataLock, err := openDataLock(dbPath + ".lock")
	if err != nil {
		return nil, err
	}
	st, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("初始化数据库: %w", err), dataLock.Close())
	}
	if err := task.ReconcileInterruptedTasks(context.Background(), st); err != nil {
		return nil, errors.Join(fmt.Errorf("恢复上次任务状态: %w", err), st.Close(), dataLock.Close())
	}

	prov := mumu.NewProvider(cfg.Provider.Executable, cfg.Timeouts.QueryTimeout)
	adbCli := adb.NewClient(cfg.Adb.Executable, cfg.Timeouts.QueryTimeout)
	adm := lifecycle.NewAdmissionController(cfg.Limits.MaxRunningInstances, cfg.Limits.StartConcurrency)
	lc := lifecycle.NewService(st, prov, adbCli, adm, cfg.Timeouts.StartReadyTimeout, cfg.Timeouts.StopTimeout)
	sm := stream.NewStreamManager(cfg.Scrcpy.Executable)
	tq := task.NewQueue(cfg.Limits.TaskConcurrency)

	appSvc := app.NewService(cfg, st, prov, adbCli, lc, sm, tq)

	return &App{
		store:      st,
		dataLock:   dataLock,
		appService: appSvc,
	}, nil
}

// startup 保存 Wails 应用上下文。
func (a *App) startup(ctx context.Context) {
	a.ctx, a.cancel = context.WithCancel(ctx)
}

func (a *App) shutdown() {
	if a.cancel != nil {
		a.cancel()
	}
	a.appService.Shutdown()
	if err := errors.Join(a.store.Close(), a.dataLock.Close()); err != nil {
		log.Printf("关闭数据库资源失败: %v", err)
	}
}

// GetSystemStatus 返回系统状态与就绪信息
func (a *App) GetSystemStatus() string {
	return "CloudMatrix 桌面服务已初始化；设备状态以刷新结果为准"
}

// Greet 示例绑定方法，用于连通性验证
func (a *App) Greet(name string) string {
	return fmt.Sprintf("你好，%s！CloudMatrix 已就绪。", name)
}

// ListInstances 查询实例列表
func (a *App) ListInstances() ([]app.InstanceDTO, error) {
	return a.appService.ListInstances(a.ctx)
}

func (a *App) RefreshInstances() ([]app.InstanceDTO, error) {
	return a.appService.RefreshInstances(a.ctx)
}

// StartInstance 启动实例
func (a *App) StartInstance(id string) (app.OperationResponse, error) {
	if a.appService == nil {
		return app.OperationResponse{Success: false, Message: "服务未就绪"}, nil
	}
	return a.appService.StartInstance(a.ctx, id)
}

// StopInstance 停止实例
func (a *App) StopInstance(id string) (app.OperationResponse, error) {
	if a.appService == nil {
		return app.OperationResponse{Success: false, Message: "服务未就绪"}, nil
	}
	return a.appService.StopInstance(a.ctx, id)
}

// OpenStream 打开投屏
func (a *App) OpenStream(id string) (app.OperationResponse, error) {
	if a.appService == nil {
		return app.OperationResponse{Success: false, Message: "服务未就绪"}, nil
	}
	return a.appService.OpenStream(a.ctx, id)
}

// CloseStream 关闭投屏
func (a *App) CloseStream(id string) (app.OperationResponse, error) {
	if a.appService == nil {
		return app.OperationResponse{Success: false, Message: "服务未就绪"}, nil
	}
	return a.appService.CloseStream(id)
}

// InstallAPK 分发安装 APK
func (a *App) InstallAPK(req app.InstallRequest) (app.OperationResponse, error) {
	if a.appService == nil {
		return app.OperationResponse{Success: false, Message: "服务未就绪"}, nil
	}
	return a.appService.InstallAPK(a.ctx, req)
}

func (a *App) GetGoodDoctorStatus(id string) (app.GoodDoctorStatusDTO, error) {
	return a.appService.GetGoodDoctorStatus(a.ctx, id)
}

func (a *App) OpenGoodDoctor(id string) (app.OperationResponse, error) {
	return a.appService.OpenGoodDoctor(a.ctx, id)
}

func (a *App) StopGoodDoctor(id string) (app.OperationResponse, error) {
	return a.appService.StopGoodDoctor(a.ctx, id)
}

func (a *App) UpdateGoodDoctorLoginStatus(req app.GoodDoctorLoginRequest) (app.GoodDoctorStatusDTO, error) {
	return a.appService.UpdateGoodDoctorLoginStatus(a.ctx, req)
}

func (a *App) GetTask(id string) (*domain.Task, error) {
	return a.appService.GetTask(a.ctx, id)
}

func (a *App) CancelTask(id string) error {
	return a.appService.CancelTask(id)
}
