package watchdog

import (
	"sync"
	"time"
)

// Policy 实例自愈保护策略配置
type Policy struct {
	Enabled              bool          `json:"enabled"`
	MaxRestartsInWindow  int           `json:"max_restarts_in_window"`
	WindowDuration       time.Duration `json:"window_duration"`
	CooldownDuration     time.Duration `json:"cooldown_duration"`
}

// DefaultPolicy 默认受限自愈策略（10分钟内至多2次，冷却60秒）
func DefaultPolicy() Policy {
	return Policy{
		Enabled:             true,
		MaxRestartsInWindow: 2,
		WindowDuration:      10 * time.Minute,
		CooldownDuration:    60 * time.Second,
	}
}

// restartRecord 单次重启时间记录
type instanceRecord struct {
	restarts    []time.Time
	lastRestart time.Time
}

// Watchdog 受限自愈看门狗控制器
type Watchdog struct {
	mu      sync.Mutex
	policy  Policy
	records map[string]*instanceRecord
}

// NewWatchdog 创建自愈看门狗实例
func NewWatchdog(p Policy) *Watchdog {
	return &Watchdog{
		policy:  p,
		records: make(map[string]*instanceRecord),
	}
}

// CanAttemptRecovery 综合评估当前是否允许对指定实例触发自动恢复
func (w *Watchdog) CanAttemptRecovery(instanceID string, isUserStopped bool, isTaskRunning bool) (canRecover bool, reason string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.policy.Enabled {
		return false, "自愈守护开关已关闭"
	}

	// 1. 用户主动停止，严禁恢复
	if isUserStopped {
		return false, "实例属于用户主动停止，禁止自愈重启"
	}

	// 2. 存在正在执行的关键任务，禁止盲目重启
	if isTaskRunning {
		return false, "实例正在执行批处理或同步任务，禁止自愈重启"
	}

	rec, ok := w.records[instanceID]
	if !ok {
		return true, "准许首次自愈恢复"
	}

	now := time.Now()

	// 3. 冷却时间检查
	if now.Sub(rec.lastRestart) < w.policy.CooldownDuration {
		return false, "仍在自愈冷却时间内，暂缓重启"
	}

	// 4. 清理超出滑动窗口的老旧记录
	windowStart := now.Add(-w.policy.WindowDuration)
	validRestarts := make([]time.Time, 0, len(rec.restarts))
	for _, t := range rec.restarts {
		if t.After(windowStart) {
			validRestarts = append(validRestarts, t)
		}
	}
	rec.restarts = validRestarts

	// 5. 预算耗尽检查（防重启风暴核心契约）
	if len(rec.restarts) >= w.policy.MaxRestartsInWindow {
		return false, "10分钟内自愈预算已耗尽 (已重启2次)，熔断转人工处理"
	}

	return true, "准许自愈恢复"
}

// RecordRecovery 记录一次成功的自愈重启操作
func (w *Watchdog) RecordRecovery(instanceID string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	rec, ok := w.records[instanceID]
	if !ok {
		rec = &instanceRecord{}
		w.records[instanceID] = rec
	}

	rec.restarts = append(rec.restarts, now)
	rec.lastRestart = now
}
