package observe

import "sync"

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusHealthy  HealthStatus = "healthy"
	HealthStatusDegraded HealthStatus = "degraded"
	HealthStatusUnknown  HealthStatus = "unknown"
)

// HealthTracker 实例健康探测计数与状态机
type HealthTracker struct {
	mu             sync.Mutex
	consecutiveErr int
	consecutiveOK  int
	currentStatus  HealthStatus
}

// NewHealthTracker 创建健康跟踪器
func NewHealthTracker() *HealthTracker {
	return &HealthTracker{
		currentStatus: HealthStatusUnknown,
	}
}

// RecordFailure 记录一次探测失败，连续失败 3 次才标记为 degraded
func (h *HealthTracker) RecordFailure() HealthStatus {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.consecutiveErr++
	h.consecutiveOK = 0

	if h.consecutiveErr >= 3 {
		h.currentStatus = HealthStatusDegraded
	}
	return h.currentStatus
}

// RecordSuccess 记录一次探测成功，连续成功 2 次才恢复为 healthy
func (h *HealthTracker) RecordSuccess() HealthStatus {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.consecutiveOK++
	h.consecutiveErr = 0

	if h.consecutiveOK >= 2 {
		h.currentStatus = HealthStatusHealthy
	}
	return h.currentStatus
}

// GetStatus 获取当前健康状态
func (h *HealthTracker) GetStatus() HealthStatus {
	h.mu.Lock()
	defer h.mu.Unlock()

	return h.currentStatus
}
