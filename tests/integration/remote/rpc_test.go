package remote_integration_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"cloudmatrix/api"
	"cloudmatrix/internal/transport"
)

// TestRPCAuthenticationAndVersionValidation 验证远程通信鉴权与协议版本匹配
func TestRPCAuthenticationAndVersionValidation(t *testing.T) {
	authToken := "MatrixSecKey#2026"
	server := transport.NewRPCServer("host-node-01", authToken, nil)
	// 动态端口监听
	if err := server.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("server.Start failed: %v", err)
	}
	defer server.Stop()

	serverAddr := server.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. 使用错误 Token 连接
	badClient := transport.NewRPCClient(serverAddr, "host-node-01", "WrongToken")
	if err := badClient.Connect(ctx); err != nil {
		t.Fatalf("badClient.Connect failed: %v", err)
	}
	defer badClient.Close()

	err := badClient.Ping(ctx)
	if err == nil {
		t.Fatal("expected unauthorized error for invalid token, got nil")
	}

	// 2. 使用正确 Token 连接
	goodClient := transport.NewRPCClient(serverAddr, "host-node-01", authToken)
	if err := goodClient.Connect(ctx); err != nil {
		t.Fatalf("goodClient.Connect failed: %v", err)
	}
	defer goodClient.Close()

	if err := goodClient.Ping(ctx); err != nil {
		t.Fatalf("goodClient.Ping failed: %v", err)
	}
}

// TestRPCIdempotentTaskExecution 验证任务执行的严格幂等与防重机制
func TestRPCIdempotentTaskExecution(t *testing.T) {
	authToken := "Token-Idempotent-Test"
	var executionCounter int32

	handler := func(ctx context.Context, req *api.TaskExecutionRequest) (*api.TaskExecutionResponse, error) {
		atomic.AddInt32(&executionCounter, 1)
		return &api.TaskExecutionResponse{
			TaskID:   req.TaskID,
			Success:  true,
			ExitCode: 0,
			Output:   fmt.Sprintf("executed %s", req.Action),
		}, nil
	}

	server := transport.NewRPCServer("host-node-02", authToken, handler)
	if err := server.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("server.Start failed: %v", err)
	}
	defer server.Stop()

	serverAddr := server.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := transport.NewRPCClient(serverAddr, "host-node-02", authToken)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("client.Connect failed: %v", err)
	}
	defer client.Close()

	taskReq := &api.TaskExecutionRequest{
		TaskID:     "task-uuid-8899",
		InstanceID: "vm-101",
		Action:     "install_apk",
		Args:       []string{"/tmp/test.apk"},
	}

	// 首次执行：必须实际调用 handler
	resp1, err := client.ExecuteTask(ctx, taskReq)
	if err != nil {
		t.Fatalf("1st ExecuteTask failed: %v", err)
	}
	if !resp1.Success || resp1.FromCache {
		t.Fatalf("1st execution should not be from cache: %+v", resp1)
	}
	if atomic.LoadInt32(&executionCounter) != 1 {
		t.Fatalf("expected handler called 1 time, got %d", executionCounter)
	}

	// 模拟网络重发相同 task_id：必须命中幂等缓存，handler 调用次数依然为 1
	resp2, err := client.ExecuteTask(ctx, taskReq)
	if err != nil {
		t.Fatalf("2nd ExecuteTask failed: %v", err)
	}
	if !resp2.Success || !resp2.FromCache {
		t.Fatalf("2nd execution must be marked as FromCache: %+v", resp2)
	}
	if atomic.LoadInt32(&executionCounter) != 1 {
		t.Fatalf("handler must NOT be executed twice! Count=%d", executionCounter)
	}

	// 通过 QueryTask 接口查询同一任务
	queryResp, err := client.QueryTask(ctx, "task-uuid-8899")
	if err != nil {
		t.Fatalf("QueryTask failed: %v", err)
	}
	if queryResp.TaskID != "task-uuid-8899" || !queryResp.Success {
		t.Fatalf("unexpected query result: %+v", queryResp)
	}
}

// TestRPCDisconnectAndReconnect 验证断线后客户端重连恢复
func TestRPCDisconnectAndReconnect(t *testing.T) {
	authToken := "Token-Reconnect-Test"
	server := transport.NewRPCServer("host-node-03", authToken, nil)
	if err := server.Start("127.0.0.1:0"); err != nil {
		t.Fatalf("server.Start failed: %v", err)
	}
	defer server.Stop()

	serverAddr := server.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := transport.NewRPCClient(serverAddr, "host-node-03", authToken)
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("initial Connect failed: %v", err)
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("initial Ping failed: %v", err)
	}

	// 模拟断线并重新连接
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("reconnected Ping failed: %v", err)
	}
}
