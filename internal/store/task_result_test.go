package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cloudmatrix/internal/domain"
)

func TestTaskResultPersistsAndRestartInterruptsOnlyPending(t *testing.T) {
	ctx := context.Background()
	st, err := NewSQLiteStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	job := &domain.Task{ID: "job", Type: "install_apk", Status: domain.TaskQueued, Targets: []*domain.TaskTarget{
		{ID: "target-1", InstanceID: "instance-1", Status: domain.TaskQueued},
		{ID: "target-2", InstanceID: "instance-2", Status: domain.TaskQueued},
	}}
	if err := st.CreateTask(ctx, job); err != nil {
		t.Fatal(err)
	}
	job.Status = domain.TaskRunning
	job.UpdatedAt = time.Now().UTC()
	job.Targets[0].Status = domain.TaskSucceeded
	job.Targets[1].Status = domain.TaskRunning
	if err := st.SaveTaskResult(ctx, job); err != nil {
		t.Fatal(err)
	}
	if err := st.InterruptPendingTasks(ctx); err != nil {
		t.Fatal(err)
	}
	saved, err := st.GetTask(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != domain.TaskInterrupted {
		t.Fatalf("task status = %s", saved.Status)
	}
	statuses := map[string]domain.TaskStatus{}
	for _, target := range saved.Targets {
		statuses[target.ID] = target.Status
	}
	if statuses["target-1"] != domain.TaskSucceeded || statuses["target-2"] != domain.TaskInterrupted {
		t.Fatalf("unexpected recovered target statuses: %v", statuses)
	}
}
