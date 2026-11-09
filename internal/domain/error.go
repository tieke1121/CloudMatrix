package domain

// 平台标准错误码枚举定义，符合架构文档规范
const (
	ErrCodeToolMissing         = "TOOL_MISSING"
	ErrCodeUnsupportedVersion  = "UNSUPPORTED_VERSION"
	ErrCodeProviderUnavailable = "PROVIDER_UNAVAILABLE"
	ErrCodeInstanceNotFound    = "INSTANCE_NOT_FOUND"
	ErrCodeAdbUnauthorized     = "ADB_UNAUTHORIZED"
	ErrCodeAdbOffline          = "ADB_OFFLINE"
	ErrCodeBootTimeout         = "BOOT_TIMEOUT"
	ErrCodeResourceLimit       = "RESOURCE_LIMIT"
	ErrCodeProcessTimeout      = "PROCESS_TIMEOUT"
	ErrCodeProcessExit         = "PROCESS_EXIT"
	ErrCodeOutputLimit         = "OUTPUT_LIMIT"
	ErrCodeCancelled           = "CANCELLED"
	ErrCodeConflict            = "CONFLICT"
	ErrCodeInvalidArgument     = "INVALID_ARGUMENT"
	ErrCodeStorageFailure      = "STORAGE_FAILURE"
)

// PlatformError 平台标准结构化错误
type PlatformError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (e *PlatformError) Error() string {
	return e.Code + ": " + e.Message
}

// NewPlatformError 快速构造标准错误
func NewPlatformError(code string, message string, retryable bool) *PlatformError {
	return &PlatformError{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}
}
