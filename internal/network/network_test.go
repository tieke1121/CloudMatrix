package network

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestCredentialRedactionAndSanitize 验证凭据脱敏安全与格式化输出
func TestCredentialRedactionAndSanitize(t *testing.T) {
	username := "cloud_admin"
	password := "SecretP@ssw0rd!123"

	// 1. 用户名脱敏
	maskedUser := RedactUsername(username)
	if maskedUser != "c***n" {
		t.Fatalf("expected c***n, got %s", maskedUser)
	}

	// 2. 密码脱敏
	maskedPass := RedactPassword(password)
	if maskedPass != "******" {
		t.Fatalf("expected ******, got %s", maskedPass)
	}

	// 3. 端点 URL 脱敏
	cfg := ProxyConfig{
		Mode:      ModeSocks5,
		Host:      "10.0.0.8",
		Port:      1080,
		Username:  username,
		Password:  password,
		RemoteDNS: true,
		BlockIPv6: true,
	}

	sanitized := SanitizeProxyEndpoint(cfg)
	if strings.Contains(sanitized, password) {
		t.Fatalf("sanitized endpoint must not contain plaintext password: %s", sanitized)
	}
	if !strings.Contains(sanitized, "socks5://c***n:******@10.0.0.8:1080") {
		t.Fatalf("unexpected sanitized string: %s", sanitized)
	}

	// 4. 配置合法性校验
	badHTTP := ProxyConfig{
		Mode:     ModeHTTP,
		Host:     "10.0.0.8",
		Port:     8080,
		BlockUDP: false, // HTTP 无法代理 UDP，未阻断 UDP 时必须被拒绝
	}
	if err := ValidateConfig(badHTTP); err == nil {
		t.Fatal("expected error for HTTP proxy without BlockUDP, got nil")
	}
}

// TestCircuitBreakerFailClosed 验证连续失败触发 Fail-Closed 与连续成功恢复
func TestCircuitBreakerFailClosed(t *testing.T) {
	blockedCalled := false
	recoveredCalled := false

	cb := NewCircuitBreaker(
		StatusProtected,
		func() { blockedCalled = true },
		func() { recoveredCalled = true },
	)

	// 首次探测失败：进入降级态，不触发阻断回调
	s1 := cb.RecordProbeResult(false)
	if s1 != StatusDegraded {
		t.Fatalf("expected StatusDegraded on 1st fail, got %s", s1)
	}
	if blockedCalled {
		t.Fatal("blocked callback should not be triggered on 1st fail")
	}

	// 第二次探测失败：连续 2 次，进入 StatusBlocked 强阻断，触发回调
	s2 := cb.RecordProbeResult(false)
	if s2 != StatusBlocked {
		t.Fatalf("expected StatusBlocked on 2nd fail, got %s", s2)
	}
	if !blockedCalled {
		t.Fatal("expected blocked callback to be called on 2nd fail")
	}

	// 恢复探测：单次成功仍在阻断态
	s3 := cb.RecordProbeResult(true)
	if s3 != StatusBlocked {
		t.Fatalf("expected still StatusBlocked on 1st recovery, got %s", s3)
	}
	if recoveredCalled {
		t.Fatal("recovered callback should not be called on 1st success")
	}

	// 第二次探测成功：连续 2 次成功，恢复 StatusProtected
	s4 := cb.RecordProbeResult(true)
	if s4 != StatusProtected {
		t.Fatalf("expected StatusProtected on 2nd recovery, got %s", s4)
	}
	if !recoveredCalled {
		t.Fatal("expected recovered callback to be called on 2nd success")
	}
}

// TestIsolationManagerLifecycleAndAdbCommands 验证网络管理器与安全 ADB 参数生成
func TestIsolationManagerLifecycleAndAdbCommands(t *testing.T) {
	// 使用可控 mock 探针
	probeState := true
	mockProbe := func(ctx context.Context, host string, port int, timeout time.Duration) bool {
		return probeState
	}

	mgr := NewIsolationManager(mockProbe)
	defer mgr.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := ProxyConfig{
		Mode:          ModeSocks5,
		Host:          "192.168.1.50",
		Port:          1080,
		BlockIPv6:     true,
		ProbeInterval: 10 * time.Millisecond,
		ProbeTimeout:  5 * time.Millisecond,
	}

	cmds, err := mgr.RegisterInstance(ctx, "vm-net-01", cfg)
	if err != nil {
		t.Fatalf("RegisterInstance failed: %v", err)
	}

	// 验证生成的参数切片
	if len(cmds) < 2 {
		t.Fatalf("expected at least 2 command sets, got %d", len(cmds))
	}
	// 验证包含代理设置和 IPv6 禁用设置
	foundProxy := false
	foundIPv6 := false
	for _, cmd := range cmds {
		for _, arg := range cmd {
			if strings.Contains(arg, "192.168.1.50:1080") {
				foundProxy = true
			}
			if strings.Contains(arg, "net.ipv6.disabled") {
				foundIPv6 = true
			}
		}
	}
	if !foundProxy || !foundIPv6 {
		t.Fatalf("missing proxy (%v) or ipv6 block (%v)", foundProxy, foundIPv6)
	}

	// 检查初始状态
	status, err := mgr.GetStatus("vm-net-01")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status != StatusProtected {
		t.Fatalf("expected StatusProtected, got %s", status)
	}

	// 手动触发 Fail-Closed
	failCmds, err := mgr.TriggerFailClosed("vm-net-01")
	if err != nil {
		t.Fatalf("TriggerFailClosed failed: %v", err)
	}
	foundBlackhole := false
	for _, cmd := range failCmds {
		for _, arg := range cmd {
			if strings.Contains(arg, "127.0.0.1:0") {
				foundBlackhole = true
			}
		}
	}
	if !foundBlackhole {
		t.Fatalf("fail-closed commands must contain blackhole proxy 127.0.0.1:0")
	}
}
