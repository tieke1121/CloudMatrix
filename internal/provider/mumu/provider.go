package mumu

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/execx"
)

// Provider MuMu 模拟器适配器
type Provider struct {
	executable string
	timeout    time.Duration
}

// NewProvider 创建 MuMu 适配器实例
func NewProvider(executable string, timeout time.Duration) *Provider {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Provider{
		executable: executable,
		timeout:    timeout,
	}
}

// Name 返回适配器名称
func (p *Provider) Name() string {
	return "mumu"
}

// Capabilities 返回适配器能力支持状态
func (p *Provider) Capabilities() *domain.ProviderCapabilities {
	return &domain.ProviderCapabilities{
		Discover: domain.CapUnverified,
		Start:    domain.CapUnverified,
		Stop:     domain.CapUnverified,
		Restart:  domain.CapUnsupported,
		Create:   domain.CapUnverified,
		Clone:    domain.CapUnverified,
		Reset:    domain.CapUnverified,
		Delete:   domain.CapUnverified,
		Headless: domain.CapUnverified,
		Metrics:  domain.CapUnverified,
	}
}

// Discover 查询宿主机上的实例列表
func (p *Provider) Discover(ctx context.Context) ([]*domain.Instance, error) {
	if p.executable == "" {
		return []*domain.Instance{}, domain.NewPlatformError(domain.ErrCodeToolMissing, "未配置 MuMu CLI 路径", false)
	}

	args := []string{"info"}
	res, err := execx.Run(ctx, p.executable, args, p.timeout, 512*1024)
	if err != nil {
		if errors.Is(err, execx.ErrToolMissing) {
			return []*domain.Instance{}, domain.NewPlatformError(domain.ErrCodeToolMissing, "MuMu CLI 工具不存在", false)
		}
		return nil, fmt.Errorf("执行 MuMu CLI 发现命令失败: %w", err)
	}

	if res.ExitCode != 0 {
		return nil, fmt.Errorf("MuMu CLI 返回非零退出码: %d, stderr: %s", res.ExitCode, string(res.Stderr))
	}

	return ParseCLIOutput(res.Stdout)
}

// Start 启动指定实例
func (p *Provider) Start(ctx context.Context, providerInstanceID string) error {
	if p.executable == "" {
		return domain.NewPlatformError(domain.ErrCodeToolMissing, "未配置 MuMu CLI 路径", false)
	}

	args, err := FormatStartCommand(providerInstanceID)
	if err != nil {
		return err
	}

	res, err := execx.Run(ctx, p.executable, args, p.timeout, 512*1024)
	if err != nil {
		return fmt.Errorf("执行 MuMu 启动命令失败: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("启动失败，退出码: %d, stderr: %s", res.ExitCode, string(res.Stderr))
	}

	return nil
}

// Stop 停止指定实例
func (p *Provider) Stop(ctx context.Context, providerInstanceID string) error {
	if p.executable == "" {
		return domain.NewPlatformError(domain.ErrCodeToolMissing, "未配置 MuMu CLI 路径", false)
	}

	args, err := FormatStopCommand(providerInstanceID)
	if err != nil {
		return err
	}

	res, err := execx.Run(ctx, p.executable, args, p.timeout, 512*1024)
	if err != nil {
		return fmt.Errorf("执行 MuMu 停止命令失败: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("停止失败，退出码: %d, stderr: %s", res.ExitCode, string(res.Stderr))
	}

	return nil
}
