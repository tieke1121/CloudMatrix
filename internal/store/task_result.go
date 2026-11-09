package store

import (
	"context"
	"fmt"
	"time"

	"cloudmatrix/internal/domain"
)

// SaveTaskResult updates the task and its targets together.
func (s *SQLiteStore) SaveTaskResult(ctx context.Context, task *domain.Task) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, "UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?", task.Status, task.UpdatedAt, task.ID)
	if err != nil {
		return fmt.Errorf("更新任务 %s: %w", task.ID, err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return fmt.Errorf("任务 %s 未更新（受影响行 %d，错误 %v）", task.ID, count, err)
	}
	for _, target := range task.Targets {
		result, err := tx.ExecContext(ctx, `UPDATE task_targets
			SET status = ?, exit_code = ?, error_code = ?, error_message = ?, started_at = ?, ended_at = ?
			WHERE task_id = ? AND id = ?`,
			target.Status, target.ExitCode, target.ErrorCode, target.ErrorMessage,
			target.StartedAt, target.EndedAt, task.ID, target.ID)
		if err != nil {
			return fmt.Errorf("更新任务目标 %s: %w", target.ID, err)
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			return fmt.Errorf("任务目标 %s 未更新（受影响行 %d，错误 %v）", target.ID, count, err)
		}
	}
	return tx.Commit()
}

// InterruptPendingTasks records tasks left unfinished by the previous process.
func (s *SQLiteStore) InterruptPendingTasks(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	const pending = "('queued', 'running', 'cancelling')"
	if _, err := tx.ExecContext(ctx, `UPDATE task_targets
		SET status = ?, exit_code = -1, error_code = 'TASK_INTERRUPTED', error_message = '上次程序退出，结果未确认', ended_at = ?
		WHERE task_id IN (SELECT id FROM tasks WHERE status IN `+pending+`) AND status IN `+pending,
		domain.TaskInterrupted, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE tasks SET status = ?, updated_at = ? WHERE status IN "+pending,
		domain.TaskInterrupted, now); err != nil {
		return err
	}
	return tx.Commit()
}
