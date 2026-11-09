package provider

import (
	"context"

	"cloudmatrix/internal/domain"
)

// Provider 模拟器厂商适配器最小接口契约
type Provider interface {
	// Name 获取适配器厂商名称 (如 "mumu")
	Name() string
	// Capabilities 返回该适配器已验证支持的能力集
	Capabilities() *domain.ProviderCapabilities
	// Discover 查询并发现宿主机上的所有实例
	Discover(ctx context.Context) ([]*domain.Instance, error)
	// Start 启动指定厂商实例
	Start(ctx context.Context, providerInstanceID string) error
	// Stop 停止指定厂商实例
	Stop(ctx context.Context, providerInstanceID string) error
}
