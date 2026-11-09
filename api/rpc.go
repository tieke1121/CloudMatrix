package api

// RPCVersion 当前版本化 RPC 契约协议版本
const RPCVersion = "v1.0"

// RPCMessage 统一 RPC 帧消息封装
type RPCMessage struct {
	Version   string `json:"version"`
	HostID    string `json:"host_id"`
	RequestID string `json:"request_id"`
	Token     string `json:"token,omitempty"`
	Action    string `json:"action"`
	Payload   []byte `json:"payload,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Action 枚举常量
const (
	ActionPing        = "ping"
	ActionRegister    = "register"
	ActionExecuteTask = "execute_task"
	ActionQueryTask   = "query_task"
)

// TaskExecutionRequest 远程任务执行请求载荷
type TaskExecutionRequest struct {
	TaskID     string   `json:"task_id"`
	InstanceID string   `json:"instance_id"`
	Action     string   `json:"action"`
	Args       []string `json:"args,omitempty"`
}

// TaskExecutionResponse 远程任务执行结果载荷
type TaskExecutionResponse struct {
	TaskID    string `json:"task_id"`
	Success   bool   `json:"success"`
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output,omitempty"`
	Error     string `json:"error,omitempty"`
	Executed  bool   `json:"executed"`
	FromCache bool   `json:"from_cache"`
}

// HostInfo 远程宿主机元数据信息
type HostInfo struct {
	HostID       string  `json:"host_id"`
	OSBuild      string  `json:"os_build"`
	CPUCores     int     `json:"cpu_cores"`
	RAMTotalGB   float64 `json:"ram_total_gb"`
	RAMAvailGB   float64 `json:"ram_avail_gb"`
	ActiveVMs    int     `json:"active_vms"`
	AgentVersion string  `json:"agent_version"`
}
