package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"strings"
	"sync"

	"cloudmatrix/api"
)

// TaskHandler 远程任务处理器签名
type TaskHandler func(ctx context.Context, req *api.TaskExecutionRequest) (*api.TaskExecutionResponse, error)

// RPCServer 运行在远程宿主上的 Host Agent 服务端
type RPCServer struct {
	hostID    string
	authToken string
	listener  net.Listener
	handler   TaskHandler
	mu        sync.Mutex
	tasks     map[string]*taskExecution
	conns     map[net.Conn]struct{}
	closed    bool
	ctx       context.Context
	cancel    context.CancelFunc
}

type taskExecution struct {
	request  api.TaskExecutionRequest
	response *api.TaskExecutionResponse
}

// NewRPCServer 创建新的 Host Agent RPC 服务端
func NewRPCServer(hostID, authToken string, handler TaskHandler) *RPCServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &RPCServer{
		hostID:    hostID,
		authToken: authToken,
		handler:   handler,
		tasks:     make(map[string]*taskExecution),
		conns:     make(map[net.Conn]struct{}),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start 在指定地址监听并提供服务
func (s *RPCServer) Start(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s failed: %w", addr, err)
	}

	s.mu.Lock()
	s.listener = l
	s.mu.Unlock()

	go s.acceptLoop()
	return nil
}

// Addr 返回当前实际监听地址
func (s *RPCServer) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// acceptLoop 连接接入循环
func (s *RPCServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			isClosed := s.closed
			s.mu.Unlock()
			if isClosed {
				return
			}
			continue
		}

		s.mu.Lock()
		s.conns[conn] = struct{}{}
		s.mu.Unlock()

		go s.handleConnection(conn)
	}
}

// handleConnection 处理单个客户端连接上的消息帧流
func (s *RPCServer) handleConnection(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
	}()

	for {
		msg, err := ReadFrame(conn)
		if err != nil {
			return
		}

		resp := s.dispatch(msg)
		if err := WriteFrame(conn, resp); err != nil {
			return
		}
	}
}

// dispatch 校验与路由分发请求
func (s *RPCServer) dispatch(req *api.RPCMessage) *api.RPCMessage {
	resp := &api.RPCMessage{
		Version:   api.RPCVersion,
		HostID:    s.hostID,
		RequestID: req.RequestID,
		Action:    req.Action,
	}

	// 1. 协议版本匹配校验
	if req.Version != api.RPCVersion {
		resp.Error = ErrVersionMismatch.Error()
		return resp
	}

	// 2. 鉴权 Token 校验
	if s.authToken != "" && req.Token != s.authToken {
		resp.Error = ErrUnauthorized.Error()
		return resp
	}

	// 3. 动作分发
	switch req.Action {
	case api.ActionPing:
		resp.Payload = []byte(`{"status":"pong"}`)
		return resp

	case api.ActionExecuteTask:
		var taskReq api.TaskExecutionRequest
		if err := json.Unmarshal(req.Payload, &taskReq); err != nil {
			resp.Error = fmt.Sprintf("invalid task payload: %v", err)
			return resp
		}

		taskResp := s.executeIdempotentTask(&taskReq)
		data, _ := json.Marshal(taskResp)
		resp.Payload = data
		return resp

	case api.ActionQueryTask:
		s.mu.Lock()
		task := s.tasks[string(req.Payload)]
		var result *api.TaskExecutionResponse
		if task != nil {
			result = task.response
		}
		s.mu.Unlock()

		if task == nil {
			resp.Error = "task not found"
			return resp
		}
		if result == nil {
			resp.Error = "task in progress"
			return resp
		}
		data, _ := json.Marshal(result)
		resp.Payload = data
		return resp

	default:
		resp.Error = fmt.Sprintf("unsupported action: %s", req.Action)
		return resp
	}
}

// executeIdempotentTask 幂等执行任务：若 task_id 已存在则直接返回缓存结果
func (s *RPCServer) executeIdempotentTask(req *api.TaskExecutionRequest) *api.TaskExecutionResponse {
	reject := func(reason string) *api.TaskExecutionResponse {
		return &api.TaskExecutionResponse{TaskID: req.TaskID, ExitCode: 1, Error: reason}
	}
	if strings.TrimSpace(req.TaskID) == "" || strings.TrimSpace(req.Action) == "" || strings.TrimSpace(req.InstanceID) == "" {
		return reject("task ID, action and instance ID are required")
	}
	if s.handler == nil {
		return reject("remote task execution is not configured")
	}

	s.mu.Lock()
	if existing := s.tasks[req.TaskID]; existing != nil {
		saved, result := existing.request, existing.response
		s.mu.Unlock()
		if saved.Action != req.Action || saved.InstanceID != req.InstanceID || !slices.Equal(saved.Args, req.Args) {
			return reject("task ID already belongs to a different request")
		}
		if result == nil {
			return reject("task in progress; query task status")
		}
		cached := *result
		cached.FromCache = true
		return &cached
	}
	execution := &taskExecution{request: *req}
	execution.request.Args = slices.Clone(req.Args)
	s.tasks[req.TaskID] = execution
	s.mu.Unlock()

	res, err := s.handler(s.ctx, req)
	if err != nil {
		res = reject(err.Error())
	} else if res == nil {
		res = reject("task handler returned no result")
	}

	result := *res
	result.TaskID = req.TaskID
	s.mu.Lock()
	execution.response = &result
	s.mu.Unlock()

	return &result
}

// Stop 优雅关闭服务端并释放所有网络端口与连接
func (s *RPCServer) Stop() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.cancel()

	if s.listener != nil {
		_ = s.listener.Close()
	}

	for conn := range s.conns {
		_ = conn.Close()
	}
	s.conns = make(map[net.Conn]struct{})
	s.mu.Unlock()
}
