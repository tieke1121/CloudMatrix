package observe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
)

// 敏感信息正则匹配模式
var (
	// passwordRegex 匹配密码、Token、Secret (支持 =、: 或空格分隔，如 token=xxx 或 token yyy)
	passwordRegex = regexp.MustCompile(`(?i)(password|passwd|secret|token|apikey|api_key|access_token|auth)(\s*[:=]\s*|\s+)([^\s&"']+)`)
	// phoneRegex 常见中国手机号格式脱敏
	phoneRegex = regexp.MustCompile(`\b(1[3-9]\d)\d{4}(\d{4})\b`)
)

// RedactString 对输入文本中的敏感数据进行脱敏保护
func RedactString(input string) string {
	if input == "" {
		return ""
	}

	// 1. 替换密码、Token 与 Secret
	redacted := passwordRegex.ReplaceAllString(input, "${1}${2}***REDACTED***")

	// 2. 替换手机号中间四位
	redacted = phoneRegex.ReplaceAllString(redacted, "${1}****${2}")

	return redacted
}

// SanitizeCommandForAudit 对审计日志生成摘要，不持久化原始秘密全文
func SanitizeCommandForAudit(command string) (redacted string, digest string) {
	redacted = RedactString(command)

	// 计算命令 SHA-256 摘要用于审计排查
	h := sha256.Sum256([]byte(command))
	digest = hex.EncodeToString(h[:])

	return redacted, fmt.Sprintf("sha256:%s", digest)
}
