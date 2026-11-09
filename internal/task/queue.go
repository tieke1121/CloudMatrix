package task

import (
	"context"
	"errors"
	"sync"
	"time"

	"cloudmatrix/internal/domain"
)

// TargetHandler 针对单个目标的执行逻辑回调
type TargetHandler func(ctx context.Context, target *domain.TaskTarget) error

// Queue 有界任务调度队列
type Queue struct {
	slots chan struct{}
}

// NewQueue 创建有界任务队列，默认并发为 2
func NewQueue(concurrency int) *Queue {
	if concurrency <= 0 {
		concurrency = 2
	}
	return &Queue{
		slots: make(chan struct{}, concurrency),
	}
}

// Execute 调度执行批量任务，保证分实例目标结果独立记录与有界并发
func (q *Queue) Execute(ctx context.Context, task *domain.Task, handler TargetHandler) *domain.Task {
	now := time.Now().UTC()
	task.Status = domain.TaskRunning
	task.UpdatedAt = now

	var wg sync.WaitGroup

	for _, target := range task.Targets {
		target := target
		target.Status = domain.TaskRunning
		targetStarted := time.Now().UTC()
		target.StartedAt = &targetStarted

		wg.Add(1)
		go func() {
			defer wg.Done()

			// 申请并发槽位
			select {
			case <-ctx.Done():
				target.Status = domain.TaskCancelled
				target.ErrorCode = domain.ErrCodeCancelled
				target.ErrorMessage = "任务执行被取消"
				targetEnded := time.Now().UTC()
				target.EndedAt = &targetEnded
				return
			case q.slots <- struct{}{}:
			}
			defer func() { <-q.slots }()

			// 检查是否已取消
			if ctx.Err() != nil {
				target.Status = domain.TaskCancelled
				target.ErrorCode = domain.ErrCodeCancelled
				target.ErrorMessage = "任务执行被取消"
				targetEnded := time.Now().UTC()
				target.EndedAt = &targetEnded
				return
			}

			// 执行目标业务逻辑
			err := handler(ctx, target)
			targetEnded := time.Now().UTC()
			target.EndedAt = &targetEnded

			if ctx.Err() != nil && err != nil {
				target.Status = domain.TaskCancelled
				target.ErrorCode = domain.ErrCodeCancelled
				target.ErrorMessage = "任务执行被取消；安装结果未确认"
			} else if err != nil {
				target.Status = domain.TaskFailed
				target.ExitCode = 1
				if target.ErrorCode == "" {
					var platformErr *domain.PlatformError
					if errors.As(err, &platformErr) {
						target.ErrorCode = platformErr.Code
					} else {
						target.ErrorCode = "EXECUTION_ERROR"
					}
				}
				target.ErrorMessage = err.Error()
			} else {
				target.Status = domain.TaskSucceeded
				target.ExitCode = 0
			}
		}()
	}

	wg.Wait()

	task.UpdatedAt = time.Now().UTC()

	// 聚合整体任务状态
	hasFailed := false
	hasCancelled := false
	for _, t := range task.Targets {
		if t.Status == domain.TaskFailed {
			hasFailed = true
		}
		if t.Status == domain.TaskCancelled {
			hasCancelled = true
		}
	}

	if hasCancelled {
		task.Status = domain.TaskCancelled
	} else if hasFailed {
		// 部分失败亦标记为主任务 Failed，由各 target 具体呈现成功/失败详情 (V07 核心契约)
		task.Status = domain.TaskFailed
	} else {
		task.Status = domain.TaskSucceeded
	}

	return task
}
