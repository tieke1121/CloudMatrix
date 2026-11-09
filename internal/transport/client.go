package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"cloudmatrix/api"
)

// RPCClient 与远程 Host Agent 进行长连接通信的客户端
type RPCClient struct {
	addr      string
	hostID    string
	authToken string
	conn      net.Conn
	mu        sync.Mutex
	reqSeq    uint64
	closed    bool
}

// NewRPCClient 创建远程 RPC 客户端
func NewRPCClient(addr, hostID, authToken string) *RPCClient {
	return &RPCClient{
		addr:      addr,
		hostID:    hostID,
		authToken: authToken,
	}
}

// Connect 建立与远程宿主机的连接
func (c *RPCClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		_ = c.conn.Close()
	}

	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return fmt.Errorf("connect to host %s failed: %w", c.addr, err)
	}

	c.conn = conn
	return nil
}

// Ping 发送探活心跳
func (c *RPCClient) Ping(ctx context.Context) error {
	reqID := fmt.Sprintf("req-%d", atomic.AddUint64(&c.reqSeq, 1))
	req := &api.RPCMessage{
		Version:   api.RPCVersion,
		HostID:    c.hostID,
		RequestID: reqID,
		Token:     c.authToken,
		Action:    api.ActionPing,
	}

	resp, err := c.sendAndReceive(req)
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

// ExecuteTask 向远程宿主机发送任务执行请求 (幂等)
func (c *RPCClient) ExecuteTask(ctx context.Context, taskReq *api.TaskExecutionRequest) (*api.TaskExecutionResponse, error) {
	data, err := json.Marshal(taskReq)
	if err != nil {
		return nil, err
	}

	reqID := fmt.Sprintf("req-%d", atomic.AddUint64(&c.reqSeq, 1))
	req := &api.RPCMessage{
		Version:   api.RPCVersion,
		HostID:    c.hostID,
		RequestID: reqID,
		Token:     c.authToken,
		Action:    api.ActionExecuteTask,
		Payload:   data,
	}

	resp, err := c.sendAndReceive(req)
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	var taskResp api.TaskExecutionResponse
	if err := json.Unmarshal(resp.Payload, &taskResp); err != nil {
		return nil, fmt.Errorf("unmarshal task response failed: %w", err)
	}

	return &taskResp, nil
}

// QueryTask 查询既有任务的执行状态 (断网重连后重查)
func (c *RPCClient) QueryTask(ctx context.Context, taskID string) (*api.TaskExecutionResponse, error) {
	reqID := fmt.Sprintf("req-%d", atomic.AddUint64(&c.reqSeq, 1))
	req := &api.RPCMessage{
		Version:   api.RPCVersion,
		HostID:    c.hostID,
		RequestID: reqID,
		Token:     c.authToken,
		Action:    api.ActionQueryTask,
		Payload:   []byte(taskID),
	}

	resp, err := c.sendAndReceive(req)
	if err != nil {
		return nil, err
	}

	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	var taskResp api.TaskExecutionResponse
	if err := json.Unmarshal(resp.Payload, &taskResp); err != nil {
		return nil, fmt.Errorf("unmarshal task response failed: %w", err)
	}

	return &taskResp, nil
}

// sendAndReceive 线程安全地发送单帧请求并等待响应
func (c *RPCClient) sendAndReceive(msg *api.RPCMessage) (*api.RPCMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, errors.New("client not connected")
	}

	if err := WriteFrame(c.conn, msg); err != nil {
		_ = c.conn.Close()
		c.conn = nil
		return nil, fmt.Errorf("write frame failed: %w", err)
	}

	resp, err := ReadFrame(c.conn)
	if err != nil {
		_ = c.conn.Close()
		c.conn = nil
		return nil, fmt.Errorf("read response frame failed: %w", err)
	}

	return resp, nil
}

// Close 关闭客户端连接
func (c *RPCClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}
