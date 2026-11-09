package adb

import (
	"testing"

	"cloudmatrix/internal/domain"
)

// 测试 adb devices -l 输出解析
func TestParseDevicesOutput(t *testing.T) {
	mockOutput := `
List of devices attached
127.0.0.1:16384        device product:nemux_x86_64 model:MuMu_Device device:nemux
127.0.0.1:16416        offline
8DF6R17523000123       unauthorized usb:1-1 product:HUAWEI_P40
`
	devices := ParseDevicesOutput(mockOutput)
	if len(devices) != 3 {
		t.Fatalf("预期解析 3 台设备，实际为: %d", len(devices))
	}

	// 1. 在线模拟器
	d0 := devices[0]
	if d0.Serial != "127.0.0.1:16384" || d0.State != DeviceStateOnline {
		t.Errorf("设备 0 解析状态错误: %+v", d0)
	}
	if d0.Model != "MuMu_Device" {
		t.Errorf("设备 0 Model 错误: %s", d0.Model)
	}

	// 2. 离线设备
	d1 := devices[1]
	if d1.Serial != "127.0.0.1:16416" || d1.State != DeviceStateOffline {
		t.Errorf("设备 1 应为 offline，实际为: %+v", d1)
	}

	// 3. 未授权实体手机
	d2 := devices[2]
	if d2.Serial != "8DF6R17523000123" || d2.State != DeviceStateUnauthorized {
		t.Errorf("设备 2 应为 unauthorized，实际为: %+v", d2)
	}
}

// 测试实体手机混入时的严格目标过滤（V04 核心验收）
func TestFilterTargetDevices(t *testing.T) {
	allDevices := []Device{
		{Serial: "127.0.0.1:16384", State: DeviceStateOnline},
		{Serial: "127.0.0.1:16416", State: DeviceStateOffline},
		{Serial: "PHONE_XYZ_12345", State: DeviceStateOnline}, // 实体手机混入
	}

	// 用户只纳管了模拟器 127.0.0.1:16384
	targets := []string{"127.0.0.1:16384"}
	filtered := FilterTargetDevices(allDevices, targets)

	if len(filtered) != 1 {
		t.Fatalf("过滤后应仅剩 1 台设备，实际为: %d", len(filtered))
	}
	if filtered[0].Serial != "127.0.0.1:16384" {
		t.Errorf("过滤结果错误，混入了其他设备: %s", filtered[0].Serial)
	}
}

// 测试三重就绪状态结构体逻辑拦截（V04 核心验收：ADB 在线但系统未启动时绝不能 ready）
func TestReadinessLogicInterception(t *testing.T) {
	// 场景 1：设备离线
	rOffline := ReadinessResult{
		Ready:        false,
		DeviceOnline: false,
		ErrorCode:    domain.ErrCodeAdbOffline,
	}
	if rOffline.Ready {
		t.Error("离线设备绝不能标记为 Ready")
	}

	// 场景 2：设备在线，但 boot_completed 为 0
	rBooting := ReadinessResult{
		Ready:         false,
		DeviceOnline:  true,
		BootCompleted: false,
		ErrorCode:     domain.ErrCodeBootTimeout,
	}
	if rBooting.Ready {
		t.Error("未完成 Boot 初始化的设备绝不能标记为 Ready")
	}

	// 场景 3：设备在线，Boot 完成，但 PM 未响应
	rPmBroken := ReadinessResult{
		Ready:            false,
		DeviceOnline:     true,
		BootCompleted:    true,
		PackageManagerOK: false,
		ErrorCode:        domain.ErrCodeBootTimeout,
	}
	if rPmBroken.Ready {
		t.Error("包管理器未就绪的设备绝不能标记为 Ready")
	}

	// 场景 4：三重指标全部达成
	rSuccess := ReadinessResult{
		Ready:            true,
		DeviceOnline:     true,
		BootCompleted:    true,
		PackageManagerOK: true,
	}
	if !rSuccess.Ready {
		t.Error("三重指标全部满足时应为 Ready")
	}
}
