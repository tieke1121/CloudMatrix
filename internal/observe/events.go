package observe

import (
	"sync"
	"sync/atomic"
	"time"
)

// Event 平台标准事件领域模型
type Event struct {
	ID         int64     `json:"id"`
	Sequence   int64     `json:"sequence"`
	InstanceID string    `json:"instance_id,omitempty"`
	TaskID     string    `json:"task_id,omitempty"`
	Type       string    `json:"type"`
	Payload    string    `json:"payload"`
	CreatedAt  time.Time `json:"created_at"`
}

// EventBus 事件流总线与快照管理
type EventBus struct {
	mu       sync.RWMutex
	sequence int64
	buffer   []Event
	maxSize  int
}

// NewEventBus 创建具备容量限制的有界事件总线
func NewEventBus(maxSize int) *EventBus {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &EventBus{
		maxSize: maxSize,
		buffer:  make([]Event, 0, maxSize),
	}
}

// Publish 发送事件并分配全局单调递增 Sequence
func (b *EventBus) Publish(instanceID string, taskID string, eventType string, payload string) Event {
	b.mu.Lock()
	defer b.mu.Unlock()

	seq := atomic.AddInt64(&b.sequence, 1)
	evt := Event{
		ID:         seq,
		Sequence:   seq,
		InstanceID: instanceID,
		TaskID:     taskID,
		Type:       eventType,
		Payload:    payload,
		CreatedAt:  time.Now().UTC(),
	}

	if len(b.buffer) >= b.maxSize {
		// 淘汰最老事件 (FIFO 有界队列)
		b.buffer = b.buffer[1:]
	}
	b.buffer = append(b.buffer, evt)

	return evt
}

// GetSnapshot 获取大于指定 sequence 的增量事件快照，支持前端断线对账
func (b *EventBus) GetSnapshot(sinceSequence int64) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []Event
	for _, e := range b.buffer {
		if e.Sequence > sinceSequence {
			result = append(result, e)
		}
	}
	return result
}
