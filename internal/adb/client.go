package adb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/execx"
)

// DeviceState ADB 设备状态
type DeviceState string

const (
	DeviceStateOnline       DeviceState = "device"
	DeviceStateOffline      DeviceState = "offline"
	DeviceStateUnauthorized DeviceState = "unauthorized"
	DeviceStateUnknown      DeviceState = "unknown"
)

// Device ADB 设备描述模型
type Device struct {
	Serial  string      `json:"serial"`
	State   DeviceState `json:"state"`
	Product string      `json:"product,omitempty"`
	Model   string      `json:"model,omitempty"`
	Device  string      `json:"device,omitempty"`
}

// Client 官方 ADB 客户端封装
type Client struct {
	executable string
	timeout    time.Duration
}

// NewClient 创建 ADB 客户端实例
func NewClient(executable string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		executable: executable,
		timeout:    timeout,
	}
}

// ParseDevicesOutput 解析 adb devices -l 输出文本
func ParseDevicesOutput(output string) []Device {
	var devices []Device
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "List of devices") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		serial := fields[0]
		stateStr := fields[1]

		var state DeviceState
		switch stateStr {
		case "device":
			state = DeviceStateOnline
		case "offline":
			state = DeviceStateOffline
		case "unauthorized":
			state = DeviceStateUnauthorized
		default:
			state = DeviceStateUnknown
		}

		d := Device{
			Serial: serial,
			State:  state,
		}

		// 解析扩展字段 (如 product:xxx model:yyy)
		for _, extra := range fields[2:] {
			parts := strings.SplitN(extra, ":", 2)
			if len(parts) == 2 {
				k, v := parts[0], parts[1]
				switch k {
				case "product":
					d.Product = v
				case "model":
					d.Model = v
				case "device":
					d.Device = v
				}
			}
		}

		devices = append(devices, d)
	}

	return devices
}

// ListDevices 获取当前所有已连接的 ADB 设备
func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	if c.executable == "" {
		return nil, domain.NewPlatformError(domain.ErrCodeToolMissing, "未配置 ADB 路径", false)
	}

	res, err := execx.Run(ctx, c.executable, []string{"devices", "-l"}, c.timeout, 512*1024)
	if err != nil {
		return nil, fmt.Errorf("执行 adb devices 失败: %w", err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("adb devices 返回非零退出码: %d", res.ExitCode)
	}

	return ParseDevicesOutput(string(res.Stdout)), nil
}

// FilterTargetDevices 严格根据指定的 serial 集合过滤设备，杜绝将非目标实体手机自动纳管
func FilterTargetDevices(all []Device, targetSerials []string) []Device {
	targetMap := make(map[string]bool)
	for _, s := range targetSerials {
		s = strings.TrimSpace(s)
		if s != "" {
			targetMap[s] = true
		}
	}

	var matched []Device
	for _, d := range all {
		if targetMap[d.Serial] {
			matched = append(matched, d)
		}
	}
	return matched
}

// RunShell 在指定设备上执行 Android shell 命令
func (c *Client) RunShell(ctx context.Context, serial string, args []string, timeout time.Duration) (*execx.Result, error) {
	if c.executable == "" {
		return nil, domain.NewPlatformError(domain.ErrCodeToolMissing, "未配置 ADB 路径", false)
	}
	if strings.TrimSpace(serial) == "" {
		return nil, domain.NewPlatformError(domain.ErrCodeInvalidArgument, "ADB serial 不能为空", false)
	}

	cmdArgs := append([]string{"-s", serial, "shell"}, args...)
	return execx.Run(ctx, c.executable, cmdArgs, timeout, 1024*1024)
}

// Install targets one explicit device and does not overwrite an existing app.
func (c *Client) Install(ctx context.Context, serial, apkPath string, timeout time.Duration) (*execx.Result, error) {
	if c.executable == "" {
		return nil, domain.NewPlatformError(domain.ErrCodeToolMissing, "未配置 ADB 路径", false)
	}
	if strings.TrimSpace(serial) == "" {
		return nil, domain.NewPlatformError(domain.ErrCodeInvalidArgument, "ADB serial 不能为空", false)
	}
	return execx.Run(ctx, c.executable, []string{"-s", serial, "install", "-R", apkPath}, timeout, 1024*1024)
}

// GetProp 读取指定系统属性
func (c *Client) GetProp(ctx context.Context, serial string, key string) (string, error) {
	res, err := c.RunShell(ctx, serial, []string{"getprop", key}, c.timeout)
	if err != nil {
		return "", err
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("获取属性 %s 退出码非零: %d", key, res.ExitCode)
	}
	return strings.TrimSpace(string(res.Stdout)), nil
}
