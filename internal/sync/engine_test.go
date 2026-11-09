package sync

import (
	"testing"
)

// 1. 验证归一化坐标向不同目标分辨率的精确映射（V14 核心验收）
func TestCoordinateMapping(t *testing.T) {
	// 目标 1：全高清 1080x1920
	targetFHD := &TargetDevice{Width: 1080, Height: 1920}
	x1, y1 := targetFHD.MapCoordinates(0.5, 0.5)
	if x1 != 540 || y1 != 960 {
		t.Errorf("FHD 坐标映射错误: 预期 (540, 960), 实际 (%d, %d)", x1, y1)
	}

	// 目标 2：标清 720x1280
	targetHD := &TargetDevice{Width: 720, Height: 1280}
	x2, y2 := targetHD.MapCoordinates(0.5, 0.5)
	if x2 != 360 || y2 != 640 {
		t.Errorf("HD 坐标映射错误: 预期 (360, 640), 实际 (%d, %d)", x2, y2)
	}

	// 越界坐标限幅保护测试
	xLimit, yLimit := targetHD.MapCoordinates(1.5, -0.5)
	if xLimit != 720 || yLimit != 0 {
		t.Errorf("越界限幅错误: 预期 (720, 0), 实际 (%d, %d)", xLimit, yLimit)
	}
}

// 2. 验证急停机制：立即拒绝新输入，清空队列并注入 UP 释放指令（V14 核心验收）
func TestEmergencyStop(t *testing.T) {
	engine := NewSyncEngine()
	engine.RegisterTarget("inst-1", "127.0.0.1:16384", 1080, 1920)

	// 正常分发
	err := engine.DispatchEvent(ActionDown, 0.2, 0.3)
	if err != nil {
		t.Fatalf("正常分发不应报错: %v", err)
	}

	// 触发用户急停
	engine.EmergencyStop()

	// 再次分发必须被立刻拒绝
	rejectErr := engine.DispatchEvent(ActionMove, 0.2, 0.4)
	if rejectErr == nil {
		t.Fatal("急停后分发新事件必须被拒绝")
	}

	// 验证目标接收到的最终事件必定是 UP 释放指令
	engine.mu.RLock()
	target := engine.targets["inst-1"]
	engine.mu.RUnlock()

	lastEvt := <-target.eventQueue
	if lastEvt.Action != ActionUp {
		t.Fatalf("急停后排空注入的指令必须为 ActionUp，实际为: %s", lastEvt.Action)
	}
}
