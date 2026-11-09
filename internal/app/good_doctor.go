package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"cloudmatrix/internal/domain"
)

const GoodDoctorPackageName = "com.haoyisheng.mobile.app"

var phoneLast4Pattern = regexp.MustCompile(`^\d{4}$`)

func normalizeLoginUpdate(req GoodDoctorLoginRequest) (domain.LoginStatus, string, error) {
	switch req.Status {
	case domain.LoginStatusNotLoggedIn, domain.LoginStatusVerificationPending, domain.LoginStatusLoggedIn, domain.LoginStatusExpired:
	default:
		return "", "", domain.NewPlatformError(domain.ErrCodeInvalidArgument, "不允许写入该登录状态", false)
	}
	last4 := strings.TrimSpace(req.PhoneLast4)
	if last4 != "" && !phoneLast4Pattern.MatchString(last4) {
		return "", "", domain.NewPlatformError(domain.ErrCodeInvalidArgument, "仅允许保存手机号后 4 位", false)
	}
	label := ""
	if last4 != "" {
		label = "****" + last4
	}
	return req.Status, label, nil
}

// GetGoodDoctorStatus reads the package manager and reconciles coarse local login state.
func (s *Service) GetGoodDoctorStatus(ctx context.Context, id string) (GoodDoctorStatusDTO, error) {
	s.instanceMu.RLock()
	defer s.instanceMu.RUnlock()
	inst, err := s.managedInstance(ctx, id)
	if err != nil {
		return GoodDoctorStatusDTO{}, err
	}
	if err := s.checkCurrentSerial(ctx, inst); err != nil {
		return GoodDoctorStatusDTO{}, err
	}
	info, err := s.adbClient.GetPackageInfo(ctx, inst.AdbSerial, GoodDoctorPackageName, s.cfg.Timeouts.ShellTimeout)
	if err != nil {
		return GoodDoctorStatusDTO{}, fmt.Errorf("查询好医生安装状态: %w", err)
	}
	session, err := s.store.GetAppSession(ctx, id, GoodDoctorPackageName)
	if err != nil {
		return GoodDoctorStatusDTO{}, err
	}
	desiredStatus := domain.LoginStatusNotInstalled
	if info.Installed {
		desiredStatus = domain.LoginStatusNotLoggedIn
	}
	dirty := false
	if session == nil {
		session = &domain.AppSession{InstanceID: id, PackageName: GoodDoctorPackageName, LoginStatus: desiredStatus}
		dirty = true
	} else if session.LoginStatus == domain.LoginStatusNotInstalled && info.Installed {
		session.LoginStatus = domain.LoginStatusNotLoggedIn
		session.AccountLabel = ""
		dirty = true
	} else if !info.Installed && session.LoginStatus != domain.LoginStatusNotInstalled {
		session.LoginStatus = domain.LoginStatusNotInstalled
		session.AccountLabel = ""
		dirty = true
	}
	if dirty {
		if err := s.store.UpsertAppSession(ctx, session); err != nil {
			return GoodDoctorStatusDTO{}, err
		}
	}
	return GoodDoctorStatusDTO{
		InstanceID: id, Installed: info.Installed, PackageName: GoodDoctorPackageName,
		VersionCode: info.VersionCode, VersionName: info.VersionName,
		LoginStatus: session.LoginStatus, AccountLabel: session.AccountLabel, UpdatedAt: session.UpdatedAt,
	}, nil
}

// OpenGoodDoctor launches the app and opens scrcpy so the user can enter the SMS code themselves.
func (s *Service) OpenGoodDoctor(ctx context.Context, id string) (OperationResponse, error) {
	s.instanceMu.RLock()
	defer s.instanceMu.RUnlock()
	inst, err := s.managedInstance(ctx, id)
	if err != nil {
		return OperationResponse{}, err
	}
	if err := s.checkCurrentSerial(ctx, inst); err != nil {
		return OperationResponse{}, err
	}
	info, err := s.adbClient.GetPackageInfo(ctx, inst.AdbSerial, GoodDoctorPackageName, s.cfg.Timeouts.ShellTimeout)
	if err != nil {
		return OperationResponse{}, err
	}
	if !info.Installed {
		return OperationResponse{}, domain.NewPlatformError(domain.ErrCodeConflict, "好医生尚未安装", false)
	}
	if err := s.adbClient.StartPackage(ctx, inst.AdbSerial, GoodDoctorPackageName, s.cfg.Timeouts.ShellTimeout); err != nil {
		return OperationResponse{}, err
	}
	result, err := s.openStream(ctx, id)
	if err != nil {
		return result, err
	}
	if !result.Success {
		return result, nil
	}
	result.Message = "好医生已启动，请在投屏窗口手动完成验证码登录"
	return result, nil
}

// StopGoodDoctor force-stops only the app process.
func (s *Service) StopGoodDoctor(ctx context.Context, id string) (OperationResponse, error) {
	s.instanceMu.RLock()
	defer s.instanceMu.RUnlock()
	inst, err := s.managedInstance(ctx, id)
	if err != nil {
		return OperationResponse{}, err
	}
	if err := s.checkCurrentSerial(ctx, inst); err != nil {
		return OperationResponse{}, err
	}
	if err := s.adbClient.ForceStopPackage(ctx, inst.AdbSerial, GoodDoctorPackageName, s.cfg.Timeouts.ShellTimeout); err != nil {
		return OperationResponse{}, err
	}
	return OperationResponse{Success: true, Message: "好医生已停止，模拟器保持运行"}, nil
}

// UpdateGoodDoctorLoginStatus records only a user-confirmed status and optional phone suffix.
func (s *Service) UpdateGoodDoctorLoginStatus(ctx context.Context, req GoodDoctorLoginRequest) (GoodDoctorStatusDTO, error) {
	status, label, err := normalizeLoginUpdate(req)
	if err != nil {
		return GoodDoctorStatusDTO{}, err
	}
	instanceID := strings.TrimSpace(req.InstanceID)
	if instanceID == "" {
		return GoodDoctorStatusDTO{}, domain.NewPlatformError(domain.ErrCodeInvalidArgument, "实例 ID 不能为空", false)
	}
	current, err := s.GetGoodDoctorStatus(ctx, instanceID)
	if err != nil {
		return GoodDoctorStatusDTO{}, err
	}
	if !current.Installed {
		return GoodDoctorStatusDTO{}, domain.NewPlatformError(domain.ErrCodeConflict, "好医生尚未安装", false)
	}
	session := &domain.AppSession{
		InstanceID: instanceID, PackageName: GoodDoctorPackageName,
		LoginStatus: status, AccountLabel: label,
	}
	if err := s.store.UpsertAppSession(ctx, session); err != nil {
		return GoodDoctorStatusDTO{}, err
	}
	current.LoginStatus = session.LoginStatus
	current.AccountLabel = session.AccountLabel
	current.UpdatedAt = session.UpdatedAt
	return current, nil
}
