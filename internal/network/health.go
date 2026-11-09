package network

import (
	"context"
	"net"
	"sync"
	"time"
)

// ProbeFunc 连通性探测函数签名
type ProbeFunc func(ctx context.Context, host string, port int, timeout time.Duration) bool

// DefaultTCPProbe 默认通过 TCP 端口拨号进行轻量连通性探活
func DefaultTCPProbe(ctx context.Context, host string, port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(host, string(rune(port)))
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// CircuitBreaker 单个实例的网络健康熔断器
type CircuitBreaker struct {
	mu           sync.Mutex
	status       NetworkStatus
	failCount    int
	successCount int
	onBlocked    func()
	onRecovered  func()
}

// NewCircuitBreaker 创建网络健康熔断器
func NewCircuitBreaker(initialStatus NetworkStatus, onBlocked, onRecovered func()) *CircuitBreaker {
	return &CircuitBreaker{
		status:      initialStatus,
		onBlocked:   onBlocked,
		onRecovered: onRecovered,
	}
}

// Status 获取当前熔断状态
func (cb *CircuitBreaker) Status() NetworkStatus {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.status
}

// RecordProbeResult 记录一次探活结果并驱动 Fail-Closed 状态机流转
func (cb *CircuitBreaker) RecordProbeResult(success bool) NetworkStatus {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		cb.failCount = 0
		cb.successCount++
		// 连续成功 2 次从阻断态或降级态恢复
		if cb.status == StatusBlocked || cb.status == StatusDegraded {
			if cb.successCount >= 2 {
				cb.status = StatusProtected
				if cb.onRecovered != nil {
					cb.onRecovered()
				}
			}
		} else {
			cb.status = StatusProtected
		}
	} else {
		cb.successCount = 0
		cb.failCount++
		// 失败 1 次进入降级预警
		if cb.failCount == 1 && cb.status != StatusBlocked {
			cb.status = StatusDegraded
		}
		// 连续失败 2 次立即触发 Fail-Closed 熔断阻断
		if cb.failCount >= 2 && cb.status != StatusBlocked {
			cb.status = StatusBlocked
			if cb.onBlocked != nil {
				cb.onBlocked()
			}
		}
	}

	return cb.status
}
