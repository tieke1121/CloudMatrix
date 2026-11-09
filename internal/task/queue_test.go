package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"cloudmatrix/internal/domain"
)

// 1. 测试三目标混合场景（一成功、一失败、一离线），验证部分失败准确呈现（V07 核心验收）
func TestQueuePartialSuccessThreeTargets(t *testing.T) {
	q := NewQueue(2)

	task := &domain.Task{
		ID:     "batch-task-001",
		Type:   "install_apk",
		Status: domain.TaskQueued,
		Targets: []*domain.TaskTarget{
			{ID: "t-1", InstanceID: "inst-success", Status: domain.TaskQueued},
			{ID: "t-2", InstanceID: "inst-fail", Status: domain.TaskQueued},
			{ID: "t-3", InstanceID: "inst-offline", Status: domain.TaskQueued},
		},
	}

	handler := func(ctx context.Context, target *domain.TaskTarget) error {
		switch target.InstanceID {
		case "inst-success":
			// 模拟成功
			return nil
		case "inst-fail":
			// 模拟安装失败
			target.ErrorCode = "INSTALL_FAILED_INVALID_APK"
			return errors.New("安装包损坏")
		case "inst-offline":
			// 模拟设备离线
			target.ErrorCode = domain.ErrCodeAdbOffline
			return errors.New("设备已离线")
		default:
			return nil
		}
	}

	result := q.Execute(context.Background(), task, handler)

	// 关键断言：部分失败时，主任务状态标记为 TaskFailed，绝不能掩盖失败 (V07 核心契约)
	if result.Status != domain.TaskFailed {
		t.Fatalf("部分失败时整体任务状态应为 failed，实际为: %s", result.Status)
	}

	// 检查 target 1 (成功)
	t1 := result.Targets[0]
	if t1.Status != domain.TaskSucceeded || t1.ExitCode != 0 {
		t.Errorf("目标 1 应成功，实际状态: %s, 退出码: %d", t1.Status, t1.ExitCode)
	}

	// 检查 target 2 (失败)
	t2 := result.Targets[1]
	if t2.Status != domain.TaskFailed || t2.ErrorCode != "INSTALL_FAILED_INVALID_APK" {
		t.Errorf("目标 2 错误码不符: %s", t2.ErrorCode)
	}

	// 检查 target 3 (离线)
	t3 := result.Targets[2]
	if t3.Status != domain.TaskFailed || t3.ErrorCode != domain.ErrCodeAdbOffline {
		t.Errorf("目标 3 离线状态不符: %s", t3.ErrorCode)
	}
}

// 2. 测试任务取消机制
func TestQueueCancellation(t *testing.T) {
	q := NewQueue(1)

	task := &domain.Task{
		ID:     "batch-cancel-001",
		Type:   "install_apk",
		Status: domain.TaskQueued,
		Targets: []*domain.TaskTarget{
			{ID: "t-slow-1", InstanceID: "inst-1", Status: domain.TaskQueued},
			{ID: "t-slow-2", InstanceID: "inst-2", Status: domain.TaskQueued},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	handler := func(ctx context.Context, target *domain.TaskTarget) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}

	result := q.Execute(ctx, task, handler)

	if result.Status != domain.TaskCancelled {
		t.Fatalf("预期整体任务被取消，实际状态: %s", result.Status)
	}
}
