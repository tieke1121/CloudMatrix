package watchdog

import (
	"testing"
	"time"
)

// 1. 验证用户主动停止与执行任务中的实例绝不自愈（V15 核心验收）
func TestWatchdogRejectsUserStoppedAndRunningTasks(t *testing.T) {
	wd := NewWatchdog(DefaultPolicy())

	// 用户主动停止场景
	can1, reason1 := wd.CanAttemptRecovery("inst-user-stop", true, false)
	if can1 {
		t.Fatalf("用户主动停止不应准许自愈，原因: %s", reason1)
	}

	// 任务执行中场景
	can2, reason2 := wd.CanAttemptRecovery("inst-task-running", false, true)
	if can2 {
		t.Fatalf("任务执行中不应准许自愈，原因: %s", reason2)
	}
}

// 2. 验证 10 分钟预算耗尽后熔断拦截（防重启风暴，V15 核心验收）
func TestWatchdogAntiStormBudgetExhaustion(t *testing.T) {
	policy := Policy{
		Enabled:             true,
		MaxRestartsInWindow: 2,
		WindowDuration:      10 * time.Minute,
		CooldownDuration:    1 * time.Millisecond, // 测试将冷却调小，专注测试预算
	}
	wd := NewWatchdog(policy)

	// 第 1 次故障，应准许
	can1, _ := wd.CanAttemptRecovery("inst-crash-1", false, false)
	if !can1 {
		t.Fatal("第 1 次异常应准许自愈")
	}
	wd.RecordRecovery("inst-crash-1")
	time.Sleep(2 * time.Millisecond)

	// 第 2 次故障，应准许
	can2, _ := wd.CanAttemptRecovery("inst-crash-1", false, false)
	if !can2 {
		t.Fatal("第 2 次异常应准许自愈")
	}
	wd.RecordRecovery("inst-crash-1")
	time.Sleep(2 * time.Millisecond)

	// 关键断言：第 3 次故障，预算已满，必须熔断拒绝！
	can3, reason3 := wd.CanAttemptRecovery("inst-crash-1", false, false)
	if can3 {
		t.Fatal("预算耗尽后必须拒绝自愈以防风暴！")
	}
	if reason3 == "" {
		t.Fatal("必须给出熔断拒绝原因")
	}
}
