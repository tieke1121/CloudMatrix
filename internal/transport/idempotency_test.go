package transport

import (
	"context"
	"sync/atomic"
	"testing"

	"cloudmatrix/api"
)

func TestConcurrentTaskReplayExecutesOnce(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	s := NewRPCServer("test", "test", func(ctx context.Context, req *api.TaskExecutionRequest) (*api.TaskExecutionResponse, error) {
		if calls.Add(1) == 1 {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
			}
		}
		return &api.TaskExecutionResponse{TaskID: req.TaskID, Success: true, Executed: true}, nil
	})
	defer s.Stop()

	req := &api.TaskExecutionRequest{TaskID: "task-1", Action: "start", InstanceID: "isolated"}
	first := make(chan *api.TaskExecutionResponse, 1)
	go func() { first <- s.executeIdempotentTask(req) }()
	<-started

	if res := s.executeIdempotentTask(req); res.Success || res.Error == "" || calls.Load() != 1 {
		t.Fatalf("in-flight replay should not invoke handler: %+v calls=%d", res, calls.Load())
	}
	conflict := s.executeIdempotentTask(&api.TaskExecutionRequest{TaskID: req.TaskID, Action: "stop", InstanceID: req.InstanceID})
	if conflict.Success || calls.Load() != 1 {
		t.Fatalf("conflicting request should fail without invoking handler: %+v calls=%d", conflict, calls.Load())
	}
	s.cancel()
	if res := <-first; !res.Success {
		t.Fatalf("first result: %+v", res)
	}
	if res := s.executeIdempotentTask(req); !res.Success || !res.FromCache || calls.Load() != 1 {
		t.Fatalf("duplicate result: %+v calls=%d", res, calls.Load())
	}
}

func TestMissingTaskHandlerCannotReportSuccess(t *testing.T) {
	s := NewRPCServer("test", "test", nil)
	defer s.Stop()
	res := s.executeIdempotentTask(&api.TaskExecutionRequest{TaskID: "task-1", Action: "start", InstanceID: "isolated"})
	if res.Success || res.Executed {
		t.Fatalf("unhandled task reported execution: %+v", res)
	}
}
