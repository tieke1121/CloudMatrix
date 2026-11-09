package network_integration_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"cloudmatrix/internal/network"
)

// TestTwoInstancesNetworkIsolationAndFailClosed 验证多实例网络出口隔离与故障阻断集成
func TestTwoInstancesNetworkIsolationAndFailClosed(t *testing.T) {
	var mu sync.Mutex
	probeResults := map[string]bool{
		"proxy-node-a": true,
		"proxy-node-b": true,
	}

	mockProbe := func(ctx context.Context, host string, port int, timeout time.Duration) bool {
		mu.Lock()
		defer mu.Unlock()
		return probeResults[host]
	}

	mgr := network.NewIsolationManager(mockProbe)
	defer mgr.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. 注册实例 A：绑定节点 A (SOCKS5 代理)
	cfgA := network.ProxyConfig{
		Mode:          network.ModeSocks5,
		Host:          "proxy-node-a",
		Port:          1080,
		Username:      "operator_alice",
		Password:      "AlicePass#999",
		RemoteDNS:     true,
		BlockIPv6:     true,
		ProbeInterval: 20 * time.Millisecond,
		ProbeTimeout:  10 * time.Millisecond,
	}
	cmdsA, err := mgr.RegisterInstance(ctx, "inst-alpha", cfgA)
	if err != nil {
		t.Fatalf("RegisterInstance inst-alpha failed: %v", err)
	}

	// 2. 注册实例 B：绑定节点 B (HTTP 代理)
	cfgB := network.ProxyConfig{
		Mode:          network.ModeHTTP,
		Host:          "proxy-node-b",
		Port:          8080,
		Username:      "operator_bob",
		Password:      "BobPass#888",
		BlockUDP:      true,
		BlockIPv6:     true,
		ProbeInterval: 20 * time.Millisecond,
		ProbeTimeout:  10 * time.Millisecond,
	}
	cmdsB, err := mgr.RegisterInstance(ctx, "inst-beta", cfgB)
	if err != nil {
		t.Fatalf("RegisterInstance inst-beta failed: %v", err)
	}

	// 3. 验证两实例的出口参数独立隔离，互不串扰
	urlA := network.SanitizeProxyEndpoint(cfgA)
	urlB := network.SanitizeProxyEndpoint(cfgB)
	if strings.Contains(urlA, "proxy-node-b") || strings.Contains(urlB, "proxy-node-a") {
		t.Fatal("instance network endpoints mixed up")
	}

	// 验证命令行参数均不包含明文密码
	for _, cmd := range append(cmdsA, cmdsB...) {
		for _, arg := range cmd {
			if strings.Contains(arg, cfgA.Password) || strings.Contains(arg, cfgB.Password) {
				t.Fatalf("leak plaintext password in ADB arguments: %s", arg)
			}
		}
	}

	// 4. 故障注入：将节点 B 标记为失联
	mu.Lock()
	probeResults["proxy-node-b"] = false
	mu.Unlock()

	// 等待探针执行 2 轮探测触发熔断
	time.Sleep(70 * time.Millisecond)

	statusB, err := mgr.GetStatus("inst-beta")
	if err != nil {
		t.Fatalf("GetStatus inst-beta failed: %v", err)
	}
	if statusB != network.StatusBlocked {
		t.Fatalf("expected inst-beta to enter StatusBlocked, got %s", statusB)
	}

	// 实例 A 应该保持受保护正常状态
	statusA, err := mgr.GetStatus("inst-alpha")
	if err != nil {
		t.Fatalf("GetStatus inst-alpha failed: %v", err)
	}
	if statusA != network.StatusProtected {
		t.Fatalf("expected inst-alpha to remain StatusProtected, got %s", statusA)
	}
}
