package network

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrInstanceNetworkNotFound 未找到对应实例的网络上下文
	ErrInstanceNetworkNotFound = errors.New("instance network context not found")
)

// InstanceNetworkContext 单个实例的网络管理上下文
type InstanceNetworkContext struct {
	InstanceID string
	Config     ProxyConfig
	Breaker    *CircuitBreaker
	Cancel     context.CancelFunc
}

// IsolationManager 管理所有实例的网络出口策略、探针与 Fail-Closed 熔断
type IsolationManager struct {
	mu        sync.RWMutex
	instances map[string]*InstanceNetworkContext
	probeFn   ProbeFunc
}

// NewIsolationManager 创建网络隔离管理器
func NewIsolationManager(probeFn ProbeFunc) *IsolationManager {
	if probeFn == nil {
		probeFn = DefaultTCPProbe
	}
	return &IsolationManager{
		instances: make(map[string]*InstanceNetworkContext),
		probeFn:   probeFn,
	}
}

// BuildAdbProxyCommands 生成针对 Android 实例的安全 ADB 命令切片 (绝不拼接 Shell)
func BuildAdbProxyCommands(cfg ProxyConfig) [][]string {
	var cmds [][]string

	switch cfg.Mode {
	case ModeFailClosed:
		// 故障强阻断：将代理指向无效的无响应黑洞端口，阻断流量溢出直连
		cmds = append(cmds, []string{"shell", "settings", "put", "global", "http_proxy", "127.0.0.1:0"})
	case ModeDirect:
		// 直连模式：清除全局代理设置
		cmds = append(cmds, []string{"shell", "settings", "put", "global", "http_proxy", ":0"})
	case ModeHTTP:
		proxyVal := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		cmds = append(cmds, []string{"shell", "settings", "put", "global", "http_proxy", proxyVal})
	case ModeSocks5:
		// SOCKS5 代理可通过全局属性设置或专有网桥
		proxyVal := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		cmds = append(cmds, []string{"shell", "setprop", "net.gprs.socks.proxy", proxyVal})
		cmds = append(cmds, []string{"shell", "settings", "put", "global", "http_proxy", proxyVal})
	}

	// 若需阻断 IPv6，下发禁用内核 IPv6 的参数
	if cfg.BlockIPv6 {
		cmds = append(cmds, []string{"shell", "setprop", "net.ipv6.disabled", "1"})
	}

	return cmds
}

// RegisterInstance 注册并配置实例的网络隔离策略
func (m *IsolationManager) RegisterInstance(parentCtx context.Context, instanceID string, cfg ProxyConfig) ([][]string, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 清理旧上下文
	if old, exists := m.instances[instanceID]; exists {
		if old.Cancel != nil {
			old.Cancel()
		}
	}

	ctx, cancel := context.WithCancel(parentCtx)

	initialStatus := StatusDirect
	if cfg.Mode == ModeFailClosed {
		initialStatus = StatusBlocked
	} else if cfg.Mode == ModeSocks5 || cfg.Mode == ModeHTTP {
		initialStatus = StatusProtected
	}

	instCtx := &InstanceNetworkContext{
		InstanceID: instanceID,
		Config:     cfg,
		Cancel:     cancel,
	}

	// 创建熔断器
	breaker := NewCircuitBreaker(
		initialStatus,
		func() {
			// 连续失败 2 次，自动触发 Fail-Closed
			m.TriggerFailClosed(instanceID)
		},
		func() {
			// 连续成功 2 次，恢复代理配置
		},
	)
	instCtx.Breaker = breaker
	m.instances[instanceID] = instCtx

	// 如果需要探活，启动后台探活 goroutine
	if cfg.Mode == ModeSocks5 || cfg.Mode == ModeHTTP {
		go m.runProbeLoop(ctx, instanceID, cfg, breaker)
	}

	// 生成初始 ADB 规则
	commands := BuildAdbProxyCommands(cfg)
	return commands, nil
}

// TriggerFailClosed 将指定实例网络立即切换为 Fail-Closed 阻断状态
func (m *IsolationManager) TriggerFailClosed(instanceID string) ([][]string, error) {
	m.mu.Lock()
	instCtx, exists := m.instances[instanceID]
	if !exists {
		m.mu.Unlock()
		return nil, ErrInstanceNetworkNotFound
	}

	instCtx.Config.Mode = ModeFailClosed
	m.mu.Unlock()

	return BuildAdbProxyCommands(instCtx.Config), nil
}

// GetStatus 获取指定实例的实时网络状态
func (m *IsolationManager) GetStatus(instanceID string) (NetworkStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instCtx, exists := m.instances[instanceID]
	if !exists {
		return "", ErrInstanceNetworkNotFound
	}
	return instCtx.Breaker.Status(), nil
}

// runProbeLoop 后台定期探测代理存活状态
func (m *IsolationManager) runProbeLoop(ctx context.Context, instanceID string, cfg ProxyConfig, cb *CircuitBreaker) {
	interval := cfg.ProbeInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	timeout := cfg.ProbeTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok := m.probeFn(ctx, cfg.Host, cfg.Port, timeout)
			cb.RecordProbeResult(ok)
		}
	}
}

// Stop 关闭管理器并停止所有探活协程
func (m *IsolationManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, inst := range m.instances {
		if inst.Cancel != nil {
			inst.Cancel()
		}
	}
	m.instances = make(map[string]*InstanceNetworkContext)
}
