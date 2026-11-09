package app

import (
	"context"
	"fmt"

	"cloudmatrix/internal/domain"
)

func (s *Service) managedInstance(ctx context.Context, id string) (*domain.Instance, error) {
	inst, err := s.store.GetInstance(ctx, id)
	if err != nil {
		return nil, err
	}
	if inst.HostID != "local" || inst.ProviderType != s.provider.Name() {
		return nil, domain.NewPlatformError(domain.ErrCodeInvalidArgument, "实例不属于本机适配器", false)
	}
	return inst, nil
}

func (s *Service) checkCurrentSerial(ctx context.Context, inst *domain.Instance) error {
	if inst.ObservedState != domain.StateReady || inst.AdbSerial == "" {
		return domain.NewPlatformError(domain.ErrCodeAdbOffline, "实例尚未就绪或未绑定 ADB", false)
	}
	found, err := s.provider.Discover(ctx)
	if err != nil {
		return fmt.Errorf("核对当前 ADB 绑定失败: %w", err)
	}
	for _, current := range found {
		if current.HostID == inst.HostID && current.ProviderType == inst.ProviderType &&
			current.ProviderInstanceID == inst.ProviderInstanceID && current.AdbSerial == inst.AdbSerial &&
			(current.ObservedState == domain.StateBooting || current.ObservedState == domain.StateReady) {
			return nil
		}
	}
	return domain.NewPlatformError(domain.ErrCodeConflict, "实例的 ADB 绑定已变化，请刷新后重试", true)
}

// RefreshInstances discovers the configured provider without starting or stopping devices.
func (s *Service) RefreshInstances(ctx context.Context) ([]InstanceDTO, error) {
	s.instanceMu.Lock()
	defer s.instanceMu.Unlock()

	found, err := s.provider.Discover(ctx)
	if err != nil {
		return nil, fmt.Errorf("发现实例失败: %w", err)
	}
	saved, err := s.store.ListInstances(ctx)
	if err != nil {
		return nil, err
	}
	previous := make(map[string]*domain.Instance)
	for _, inst := range saved {
		if inst.HostID == "local" && inst.ProviderType == s.provider.Name() {
			previous[inst.ProviderInstanceID] = inst
		}
	}
	for _, inst := range found {
		if inst.HostID != "local" || inst.ProviderType != s.provider.Name() {
			return nil, fmt.Errorf("适配器返回了不属于本机的实例")
		}
		if prior := previous[inst.ProviderInstanceID]; prior != nil {
			inst.DesiredState = prior.DesiredState
			delete(previous, inst.ProviderInstanceID)
		}
		if inst.ObservedState == domain.StateBooting && inst.AdbSerial != "" && s.adbClient != nil {
			probeCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeouts.QueryTimeout)
			ready, probeErr := s.adbClient.CheckReadiness(probeCtx, inst.AdbSerial)
			cancel()
			if probeErr == nil && ready.Ready {
				inst.ObservedState = domain.StateReady
			}
		}
		if _, err := s.store.UpsertInstance(ctx, inst); err != nil {
			return nil, err
		}
	}
	for _, inst := range previous {
		inst.ObservedState = domain.StateUnknown
		inst.AdbSerial = ""
		if _, err := s.store.UpsertInstance(ctx, inst); err != nil {
			return nil, err
		}
	}
	return s.ListInstances(ctx)
}
