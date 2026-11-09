package domain

import "time"

// TaskStatus 任务运行状态
type TaskStatus string

const (
	TaskQueued      TaskStatus = "queued"
	TaskRunning     TaskStatus = "running"
	TaskSucceeded   TaskStatus = "succeeded"
	TaskFailed      TaskStatus = "failed"
	TaskCancelling  TaskStatus = "cancelling"
	TaskCancelled   TaskStatus = "cancelled"
	TaskInterrupted TaskStatus = "interrupted"
)

// TaskTarget 单实例执行目标明细结果
type TaskTarget struct {
	ID           string     `json:"id"`
	TaskID       string     `json:"task_id"`
	InstanceID   string     `json:"instance_id"`
	Status       TaskStatus `json:"status"`
	ExitCode     int        `json:"exit_code"`
	ErrorCode    string     `json:"error_code,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}

// Task 异步批量任务领域模型
type Task struct {
	ID             string        `json:"id"`
	Type           string        `json:"type"`
	Status         TaskStatus    `json:"status"`
	IdempotencyKey string        `json:"idempotency_key"`
	Targets        []*TaskTarget `json:"targets"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}
