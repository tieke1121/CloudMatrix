package observe

import (
	"strings"
	"testing"
)

// 测试敏感凭据脱敏
func TestRedactString(t *testing.T) {
	input := "login --user admin --password=SuperSecretPassword123 --token=my_secret_token_abc"
	redacted := RedactString(input)

	if strings.Contains(redacted, "SuperSecretPassword123") {
		t.Error("未成功脱敏密码参数")
	}
	if strings.Contains(redacted, "my_secret_token_abc") {
		t.Error("未成功脱敏 token 参数")
	}
	if !strings.Contains(redacted, "password=***REDACTED***") {
		t.Errorf("脱敏格式不符: %s", redacted)
	}

	phoneInput := "联系电话: 13812345678"
	phoneRedacted := RedactString(phoneInput)
	if strings.Contains(phoneRedacted, "1234") {
		t.Error("未成功脱敏手机号中间位")
	}
	if !strings.Contains(phoneRedacted, "138****5678") {
		t.Errorf("手机号脱敏格式不符: %s", phoneRedacted)
	}
}

// 测试审计摘要计算
func TestSanitizeCommandForAudit(t *testing.T) {
	cmd := "setprop debug.token secret_val_xyz"
	redacted, digest := SanitizeCommandForAudit(cmd)

	if strings.Contains(redacted, "secret_val_xyz") {
		t.Error("命令脱敏未生效")
	}
	if !strings.HasPrefix(digest, "sha256:") || len(digest) < 10 {
		t.Errorf("生成的审计摘要不合法: %s", digest)
	}
}
