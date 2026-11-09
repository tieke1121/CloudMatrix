package observe

import (
	"fmt"
	"time"
)

// InstanceMetrics 实例性能采样指标模型
type InstanceMetrics struct {
	InstanceID            string    `json:"instance_id"`
	SampleTime            time.Time `json:"sample_time"`
	CPUPercent            float64   `json:"cpu_percent"`
	MemoryWorkingSetBytes uint64    `json:"memory_working_set_bytes"`
	// Missing 进程是否缺失或无法关联，无法关联时为 true 并呈现 unknown，绝不能输出 CPU=0 冒充空闲
	Missing bool `json:"missing"`
}

// FormatCPUDisplay 格式化 CPU 显示文本，遵循 V09 契约规范
func (m *InstanceMetrics) FormatCPUDisplay() string {
	if m.Missing {
		return "unknown"
	}
	return fmt.Sprintf("%.1f%%", m.CPUPercent)
}
