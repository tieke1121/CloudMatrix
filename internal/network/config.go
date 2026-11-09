package network

import "time"

// IsolationMode 实例网络隔离模式
type IsolationMode string

const (
	// ModeDirect 直连宿主网络模式 (不启用隔离)
	ModeDirect IsolationMode = "direct"
	// ModeSocks5 SOCKS5 代理隔离模式 (支持 TCP/UDP/远端DNS)
	ModeSocks5 IsolationMode = "socks5"
	// ModeHTTP HTTP/HTTPS 代理隔离模式 (仅支持 TCP CONNECT)
	ModeHTTP IsolationMode = "http"
	// ModeFailClosed 故障强阻断模式 (彻底切断实例网络，防止直连泄漏)
	ModeFailClosed IsolationMode = "fail_closed"
)

// NetworkStatus 实例当前网络状态
type NetworkStatus string

const (
	// StatusDirect 已直连宿主
	StatusDirect NetworkStatus = "direct"
	// StatusProtected 隔离生效中，流量受控
	StatusProtected NetworkStatus = "protected"
	// StatusDegraded 代理不稳定，预警中
	StatusDegraded NetworkStatus = "degraded"
	// StatusBlocked 代理失效触发 Fail-Closed，实例流量已被物理切断
	StatusBlocked NetworkStatus = "blocked"
)

// ProxyConfig 实例网络隔离代理配置
type ProxyConfig struct {
	Mode          IsolationMode `json:"mode"`
	Host          string        `json:"host"`
	Port          int           `json:"port"`
	Username      string        `json:"username,omitempty"`
	Password      string        `json:"-"`
	RemoteDNS     bool          `json:"remote_dns"`
	BlockUDP      bool          `json:"block_udp"`
	BlockIPv6     bool          `json:"block_ipv6"`
	ProbeInterval time.Duration `json:"probe_interval"`
	ProbeTimeout  time.Duration `json:"probe_timeout"`
}

// DefaultProxyConfig 返回具有安全 Fail-Closed 保护的默认代理配置
func DefaultProxyConfig(mode IsolationMode, host string, port int) ProxyConfig {
	return ProxyConfig{
		Mode:          mode,
		Host:          host,
		Port:          port,
		RemoteDNS:     true,
		BlockUDP:      mode == ModeHTTP,
		BlockIPv6:     true,
		ProbeInterval: 5 * time.Second,
		ProbeTimeout:  2 * time.Second,
	}
}
