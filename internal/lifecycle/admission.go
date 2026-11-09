package lifecycle

import (
	"context"
	"fmt"
	"sync"

	"cloudmatrix/internal/domain"
)

// AdmissionController 资源准入与启动并发控制器
type AdmissionController struct {
	mu                  sync.Mutex
	maxRunningInstances int
	runningInstances    map[string]bool
	startSem            chan struct{}
}

// NewAdmissionController 创建资源准入控制器
func NewAdmissionController(maxRunning int, startConcurrency int) *AdmissionController {
	if maxRunning <= 0 {
		maxRunning = 5
	}
	if startConcurrency <= 0 {
		startConcurrency = 1
	}

	return &AdmissionController{
		maxRunningInstances: maxRunning,
		runningInstances:    make(map[string]bool),
		startSem:            make(chan struct{}, startConcurrency),
	}
}

// AcquireStartSlot 申请启动队列并发槽位
func (a *AdmissionController) AcquireStartSlot(ctx context.Context) (func(), error) {
	select {
	case <-ctx.Done():
		return nil, domain.NewPlatformError(domain.ErrCodeCancelled, "等待启动排队被取消", false)
	case a.startSem <- struct{}{}:
		released := false
		return func() {
			if !released {
				released = true
				<-a.startSem
			}
		}, nil
	}
}

// AcquireRunningQuota 检查并原子锁定运行实例配额
func (a *AdmissionController) AcquireRunningQuota(instanceID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.runningInstances[instanceID] {
		// 已经计入运行配额
		return nil
	}

	if len(a.runningInstances) >= a.maxRunningInstances {
		return domain.NewPlatformError(
			domain.ErrCodeResourceLimit,
			fmt.Sprintf("已达到最大运行实例数上限: %d，拒绝启动新实例", a.maxRunningInstances),
			true,
		)
	}

	a.runningInstances[instanceID] = true
	return nil
}

// ReleaseRunningQuota 释放运行配额
func (a *AdmissionController) ReleaseRunningQuota(instanceID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.runningInstances, instanceID)
}

// ObserveRunning accounts for an instance already started outside this application.
func (a *AdmissionController) ObserveRunning(instanceID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.runningInstances[instanceID] = true
}

// GetRunningCount 获取当前运行中的实例数
func (a *AdmissionController) GetRunningCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()

	return len(a.runningInstances)
}
