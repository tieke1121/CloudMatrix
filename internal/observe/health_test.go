package observe

import (
	"testing"
)

// 1. 验证连续失败 3 次才标记异常，恢复需连续成功 2 次（T09 核心验收）
func TestHealthTrackerHysteresis(t *testing.T) {
	tracker := NewHealthTracker()

	// 失败 1 次
	s1 := tracker.RecordFailure()
	if s1 == HealthStatusDegraded {
		t.Error("仅失败 1 次不应标记为 Degraded")
	}

	// 失败 2 次
	s2 := tracker.RecordFailure()
	if s2 == HealthStatusDegraded {
		t.Error("仅失败 2 次不应标记为 Degraded")
	}

	// 失败 3 次，必须流转为 Degraded
	s3 := tracker.RecordFailure()
	if s3 != HealthStatusDegraded {
		t.Errorf("连续失败 3 次应标记为 Degraded，实际为: %s", s3)
	}

	// 成功 1 次，仍应保持 Degraded (防止抖动)
	s4 := tracker.RecordSuccess()
	if s4 != HealthStatusDegraded {
		t.Errorf("仅成功 1 次不应立即恢复，实际为: %s", s4)
	}

	// 连续成功 2 次，恢复为 Healthy
	s5 := tracker.RecordSuccess()
	if s5 != HealthStatusHealthy {
		t.Errorf("连续成功 2 次应恢复为 Healthy，实际为: %s", s5)
	}
}

// 2. 验证无法关联进程时指标展示 unknown，而非 CPU=0 冒充正常（V09 核心验收）
func TestMetricsFormatUnknownOnMissing(t *testing.T) {
	mMissing := InstanceMetrics{
		InstanceID: "inst-missing",
		CPUPercent: 0.0,
		Missing:    true,
	}

	display := mMissing.FormatCPUDisplay()
	if display != "unknown" {
		t.Fatalf("缺失进程指标必须展示 unknown，实际为: %s", display)
	}

	mNormal := InstanceMetrics{
		InstanceID: "inst-normal",
		CPUPercent: 12.5,
		Missing:    false,
	}
	if mNormal.FormatCPUDisplay() != "12.5%" {
		t.Fatalf("正常指标展示错误: %s", mNormal.FormatCPUDisplay())
	}
}
