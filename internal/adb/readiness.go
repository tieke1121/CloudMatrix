package adb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloudmatrix/internal/domain"
)

// ReadinessResult 三重就绪检测结果
type ReadinessResult struct {
	Ready           bool   `json:"ready"`
	DeviceOnline    bool   `json:"device_online"`
	BootCompleted   bool   `json:"boot_completed"`
	PackageManagerOK bool   `json:"package_manager_ok"`
	ErrorCode       string `json:"error_code,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// CheckReadiness 严格执行三重就绪探测：ADB 授权可达 + Android boot 完成 + 包管理器响应
func (c *Client) CheckReadiness(ctx context.Context, serial string) (*ReadinessResult, error) {
	result := &ReadinessResult{
		Ready: false,
	}

	if strings.TrimSpace(serial) == "" {
		result.ErrorCode = domain.ErrCodeInvalidArgument
		result.Reason = "设备 serial 为空"
		return result, nil
	}

	// 1. 检查 ADB 连接与授权状态
	devices, err := c.ListDevices(ctx)
	if err != nil {
		result.ErrorCode = domain.ErrCodeToolMissing
		result.Reason = fmt.Sprintf("查询 ADB 设备列表失败: %v", err)
		return result, err
	}

	var matchedState DeviceState = DeviceStateUnknown
	found := false
	for _, d := range devices {
		if d.Serial == serial {
			matchedState = d.State
			found = true
			break
		}
	}

	if !found {
		result.ErrorCode = domain.ErrCodeInstanceNotFound
		result.Reason = fmt.Sprintf("ADB 列表中未发现设备 %s", serial)
		return result, nil
	}

	if matchedState == DeviceStateUnauthorized {
		result.ErrorCode = domain.ErrCodeAdbUnauthorized
		result.Reason = fmt.Sprintf("设备 %s 处于未授权状态 (unauthorized)", serial)
		return result, nil
	}

	if matchedState != DeviceStateOnline {
		result.ErrorCode = domain.ErrCodeAdbOffline
		result.Reason = fmt.Sprintf("设备 %s 处于离线状态 (%s)", serial, matchedState)
		return result, nil
	}
	result.DeviceOnline = true

	// 2. 检查 Android 启动标志 (sys.boot_completed)
	bootVal, bootErr := c.GetProp(ctx, serial, "sys.boot_completed")
	if bootErr != nil || strings.TrimSpace(bootVal) != "1" {
		result.ErrorCode = domain.ErrCodeBootTimeout
		result.Reason = fmt.Sprintf("系统尚未完全就绪 (sys.boot_completed=%s)", bootVal)
		return result, nil
	}
	result.BootCompleted = true

	// 3. 检查包管理器是否可正常响应
	pmRes, pmErr := c.RunShell(ctx, serial, []string{"pm", "path", "android"}, 5*time.Second)
	if pmErr != nil || pmRes.ExitCode != 0 || !strings.Contains(string(pmRes.Stdout), "package:") {
		result.ErrorCode = domain.ErrCodeBootTimeout
		result.Reason = "Android 包管理器尚未就绪"
		return result, nil
	}
	result.PackageManagerOK = true

	// 三重检测全部通过，方可标记为 ready
	result.Ready = true
	return result, nil
}
