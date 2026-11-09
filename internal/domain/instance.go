package domain

import "time"

// InstanceState 实例状态类型
type InstanceState string

const (
	StateDiscovered InstanceState = "discovered"
	StateStopped    InstanceState = "stopped"
	StateStarting   InstanceState = "starting"
	StateBooting    InstanceState = "booting"
	StateReady      InstanceState = "ready"
	StateStopping   InstanceState = "stopping"
	StateError      InstanceState = "error"
	StateUnknown    InstanceState = "unknown"
)

// Instance 核心实例领域模型，包含期望状态与观测状态
type Instance struct {
	// ID 平台内部唯一标识 (UUID)
	ID string `json:"id"`
	// HostID 宿主机标识，单机默认为 local
	HostID string `json:"host_id"`
	// ProviderType 模拟器厂商类型 (如 mumu)
	ProviderType string `json:"provider_type"`
	// ProviderInstanceID 厂商内部实例唯一标识 (非主键，如 mumu 索引或名称)
	ProviderInstanceID string `json:"provider_instance_id"`
	// Name 显示名称
	Name string `json:"name"`
	// DesiredState 期望达到的目标状态
	DesiredState InstanceState `json:"desired_state"`
	// ObservedState 实际观测到的底层状态
	ObservedState InstanceState `json:"observed_state"`
	// AdbSerial 绑定的 ADB 设备序号 (如 127.0.0.1:16384)
	AdbSerial string `json:"adb_serial"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}
