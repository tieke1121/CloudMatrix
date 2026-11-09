package stream

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"cloudmatrix/internal/domain"
)

// Session 单实例投屏会话
type Session struct {
	InstanceID string             `json:"instance_id"`
	Serial     string             `json:"serial"`
	StartedAt  time.Time          `json:"started_at"`
	cancel     context.CancelFunc `json:"-"`
	cmd        *exec.Cmd          `json:"-"`
	done       chan struct{}      `json:"-"`
}

// Manager 投屏会话管理接口
type Manager interface {
	StartSession(ctx context.Context, instanceID string, opts ScrcpyOptions) (*Session, error)
	StopSession(instanceID string) error
	HasSession(instanceID string) bool
	ListSessions() []*Session
}

// StreamManager 独立投屏会话管理器实现
type StreamManager struct {
	scrcpyPath string
	sessions   sync.Map
	mu         sync.Mutex
}

// NewStreamManager 创建投屏会话管理器
func NewStreamManager(scrcpyPath string) *StreamManager {
	return &StreamManager{
		scrcpyPath: scrcpyPath,
	}
}

// StartSession 启动独立 scrcpy 投屏窗口，每实例至多存在一个活跃会话
func (m *StreamManager) StartSession(ctx context.Context, instanceID string, opts ScrcpyOptions) (*Session, error) {
	if instanceID == "" {
		return nil, domain.NewPlatformError(domain.ErrCodeInvalidArgument, "实例 ID 不能为空", false)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. 若已有活跃会话，直接返回已有会话（单例会话保证）
	if val, ok := m.sessions.Load(instanceID); ok {
		return val.(*Session), nil
	}

	// 2. 校验可执行文件路径
	if m.scrcpyPath == "" {
		return nil, domain.NewPlatformError(domain.ErrCodeToolMissing, "未配置 scrcpy 可执行路径", false)
	}
	if _, err := os.Stat(m.scrcpyPath); err != nil {
		if _, lookErr := exec.LookPath(m.scrcpyPath); lookErr != nil {
			return nil, domain.NewPlatformError(domain.ErrCodeToolMissing, fmt.Sprintf("scrcpy 工具不存在: %s", m.scrcpyPath), false)
		}
	}

	// 3. 构建参数
	args, err := BuildScrcpyArgs(opts)
	if err != nil {
		return nil, domain.NewPlatformError(domain.ErrCodeInvalidArgument, err.Error(), false)
	}

	// 4. 创建带取消的上下文
	sessionCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(sessionCtx, m.scrcpyPath, args...)

	if startErr := cmd.Start(); startErr != nil {
		cancel()
		return nil, fmt.Errorf("启动 scrcpy 进程失败: %w", startErr)
	}

	session := &Session{
		InstanceID: instanceID,
		Serial:     opts.Serial,
		StartedAt:  time.Now(),
		cancel:     cancel,
		cmd:        cmd,
		done:       make(chan struct{}),
	}

	m.sessions.Store(instanceID, session)

	// 5. 异步观测进程生命周期：进程退出时仅清理会话资源，绝不影响模拟器实例本身 (V06 验收核心)
	go func() {
		defer func() {
			m.sessions.CompareAndDelete(instanceID, session)
			cancel()
			close(session.done)
		}()
		_ = cmd.Wait()
	}()

	return session, nil
}

// StopSession 主动终止指定实例的投屏会话
func (m *StreamManager) StopSession(instanceID string) error {
	val, ok := m.sessions.Load(instanceID)
	if !ok {
		return nil
	}

	session := val.(*Session)
	session.cancel()

	// 等待进程退出
	select {
	case <-session.done:
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("投屏进程未在 3 秒内退出")
	}
}

// HasSession 查询指定实例当前是否存在投屏会话
func (m *StreamManager) HasSession(instanceID string) bool {
	_, ok := m.sessions.Load(instanceID)
	return ok
}

// ListSessions 获取当前所有活跃投屏会话快照
func (m *StreamManager) ListSessions() []*Session {
	var list []*Session
	m.sessions.Range(func(key, value interface{}) bool {
		list = append(list, value.(*Session))
		return true
	})
	return list
}
