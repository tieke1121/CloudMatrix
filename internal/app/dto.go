package app

import (
	"time"

	"cloudmatrix/internal/domain"
)

// InstanceDTO 前端展示实例数据传输对象
type InstanceDTO struct {
	ID                 string               `json:"id"`
	HostID             string               `json:"host_id"`
	ProviderType       string               `json:"provider_type"`
	ProviderInstanceID string               `json:"provider_instance_id"`
	Name               string               `json:"name"`
	DesiredState       domain.InstanceState `json:"desired_state"`
	ObservedState      domain.InstanceState `json:"observed_state"`
	AdbSerial          string               `json:"adb_serial"`
	HasStream          bool                 `json:"has_stream"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
}

// InstallRequest APK 分发安装请求
type InstallRequest struct {
	APKPath     string   `json:"apk_path"`
	InstanceIDs []string `json:"instance_ids"`
}

// GoodDoctorStatusDTO exposes installation identity and user-confirmed login state.
type GoodDoctorStatusDTO struct {
	InstanceID   string             `json:"instance_id"`
	Installed    bool               `json:"installed"`
	PackageName  string             `json:"package_name"`
	VersionCode  string             `json:"version_code"`
	VersionName  string             `json:"version_name"`
	LoginStatus  domain.LoginStatus `json:"login_status"`
	AccountLabel string             `json:"account_label"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

// GoodDoctorLoginRequest contains no phone number, SMS code, password, cookie, or token.
type GoodDoctorLoginRequest struct {
	InstanceID string             `json:"instance_id"`
	Status     domain.LoginStatus `json:"status"`
	PhoneLast4 string             `json:"phone_last4"`
}

// CommandExecutionRequest 控制台诊断命令请求
type CommandExecutionRequest struct {
	Action      string   `json:"action"`
	Command     string   `json:"command"`
	InstanceIDs []string `json:"instance_ids"`
}

// OperationResponse 统一操作结果返回
type OperationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	TaskID  string `json:"task_id,omitempty"`
}
