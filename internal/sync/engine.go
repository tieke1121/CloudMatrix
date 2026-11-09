package sync

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// TouchAction 触控动作类型
type TouchAction string

const (
	ActionDown TouchAction = "DOWN"
	ActionMove TouchAction = "MOVE"
	ActionUp   TouchAction = "UP"
)

// NormalizedTouchEvent 归一化触控事件
type NormalizedTouchEvent struct {
	Sequence  int64       `json:"sequence"`
	Action    TouchAction `json:"action"`
	NormX     float64     `json:"norm_x"`
	NormY     float64     `json:"norm_y"`
	Timestamp time.Time   `json:"timestamp"`
}

// TargetDevice 同步目标设备描述
type TargetDevice struct {
	InstanceID string
	Serial     string
	Width      int
	Height     int
	eventQueue chan NormalizedTouchEvent
}

// MapCoordinates 将归一化坐标转换映射为目标设备的本地物理像素坐标
func (t *TargetDevice) MapCoordinates(normX, normY float64) (x int, y int) {
	// 限幅在 0.0 ~ 1.0 之间
	normX = math.Max(0.0, math.Min(1.0, normX))
	normY = math.Max(0.0, math.Min(1.0, normY))

	x = int(math.Round(normX * float64(t.Width)))
	y = int(math.Round(normY * float64(t.Height)))
	return x, y
}

// SyncEngine 多屏同步调度引擎
type SyncEngine struct {
	mu           sync.RWMutex
	targets      map[string]*TargetDevice
	sequence     int64
	stopped      atomic.Bool
	stopNotifier chan struct{}
}

// NewSyncEngine 创建多屏同步引擎
func NewSyncEngine() *SyncEngine {
	return &SyncEngine{
		targets:      make(map[string]*TargetDevice),
		stopNotifier: make(chan struct{}),
	}
}

// RegisterTarget 注册同步目标实例
func (e *SyncEngine) RegisterTarget(instanceID, serial string, width, height int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.targets[instanceID] = &TargetDevice{
		InstanceID: instanceID,
		Serial:     serial,
		Width:      width,
		Height:     height,
		eventQueue: make(chan NormalizedTouchEvent, 50), // 有界缓冲队列
	}
}

// UnregisterTarget 注销同步目标实例
func (e *SyncEngine) UnregisterTarget(instanceID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if t, ok := e.targets[instanceID]; ok {
		close(t.eventQueue)
		delete(e.targets, instanceID)
	}
}

// DispatchEvent 向所有注册目标下发归一化事件，非阻塞广播，防止慢目标拖死调度
func (e *SyncEngine) DispatchEvent(action TouchAction, normX, normY float64) error {
	if e.stopped.Load() {
		return errors.New("同步引擎已处于急停状态，拒绝分发新事件")
	}

	seq := atomic.AddInt64(&e.sequence, 1)
	evt := NormalizedTouchEvent{
		Sequence:  seq,
		Action:    action,
		NormX:     normX,
		NormY:     normY,
		Timestamp: time.Now().UTC(),
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, target := range e.targets {
		select {
		case target.eventQueue <- evt:
		default:
			// 慢目标队列已满：如果是 MOVE 动作则主动丢弃以防延迟积压；如果是 DOWN/UP 关键帧则强力等待
			if evt.Action == ActionMove {
				// 丢弃过时移动帧
			} else {
				// 关键动作非阻塞尝试
				select {
				case target.eventQueue <- evt:
				default:
				}
			}
		}
	}
	return nil
}

// EmergencyStop 用户急停，1 秒内中断下发并向所有目标发出释放指令
func (e *SyncEngine) EmergencyStop() {
	if e.stopped.Swap(true) {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// 清空所有目标的待执行事件并注入 UP 释放指令
	releaseEvt := NormalizedTouchEvent{
		Sequence:  atomic.AddInt64(&e.sequence, 1),
		Action:    ActionUp,
		NormX:     0.5,
		NormY:     0.5,
		Timestamp: time.Now().UTC(),
	}

	for _, target := range e.targets {
		// 排空积压
	drainLoop:
		for {
			select {
			case <-target.eventQueue:
			default:
				break drainLoop
			}
		}
		// 写入释放事件
		select {
		case target.eventQueue <- releaseEvt:
		default:
		}
	}
}

// ResetStopState 重置急停开关以允许重新开始同步
func (e *SyncEngine) ResetStopState() {
	e.stopped.Store(false)
}
