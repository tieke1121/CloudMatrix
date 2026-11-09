package task

import (
	"context"

	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/store"
)

// ReconcileInterruptedTasks 在服务重启后将遗留的 running/cancelling 任务标为 interrupted
func ReconcileInterruptedTasks(ctx context.Context, st store.Store) error {
	return st.InterruptPendingTasks(ctx)
}

// MarkTargetInterrupted 标记单目标为中断
func MarkTargetInterrupted(target *domain.TaskTarget) {
	if target.Status == domain.TaskRunning || target.Status == domain.TaskQueued {
		target.Status = domain.TaskInterrupted
		target.ErrorCode = "TASK_INTERRUPTED"
		target.ErrorMessage = "宿主应用退出导致任务中断"
	}
}
