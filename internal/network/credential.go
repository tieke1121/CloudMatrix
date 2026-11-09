package network

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// RedactUsername 对用户名进行脱敏显示 (保留首尾字符)
func RedactUsername(username string) string {
	if len(username) == 0 {
		return ""
	}
	if len(username) <= 2 {
		return "***"
	}
	return fmt.Sprintf("%c***%c", username[0], username[len(username)-1])
}

// RedactPassword 对密码进行彻底遮蔽，绝不输出明文
func RedactPassword(password string) string {
	if len(password) == 0 {
		return ""
	}
	return "******"
}

// CredentialDigest 计算凭据哈希摘要，用于审计比对而无需存储明文密码
func CredentialDigest(username, password string) string {
	h := sha256.New()
	h.Write([]byte(username + ":" + password))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// SanitizeProxyEndpoint 返回脱敏后的代理连接展示串 (如 socks5://u***r:******@192.168.1.100:1080)
func SanitizeProxyEndpoint(cfg ProxyConfig) string {
	if cfg.Mode == ModeDirect {
		return "direct://local"
	}
	if cfg.Mode == ModeFailClosed {
		return "fail_closed://blocked"
	}

	hostPort := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	if cfg.Username == "" && cfg.Password == "" {
		return fmt.Sprintf("%s://%s", cfg.Mode, hostPort)
	}

	maskedUser := RedactUsername(cfg.Username)
	maskedPass := RedactPassword(cfg.Password)
	return fmt.Sprintf("%s://%s:%s@%s", cfg.Mode, maskedUser, maskedPass, hostPort)
}

// ValidateConfig 检查代理配置的有效性与安全红线
func ValidateConfig(cfg ProxyConfig) error {
	switch cfg.Mode {
	case ModeDirect, ModeFailClosed:
		return nil
	case ModeSocks5, ModeHTTP:
		if strings.TrimSpace(cfg.Host) == "" {
			return fmt.Errorf("proxy host cannot be empty")
		}
		if cfg.Port <= 0 || cfg.Port > 65535 {
			return fmt.Errorf("invalid proxy port: %d", cfg.Port)
		}
		// HTTP 模式下必须阻断 UDP
		if cfg.Mode == ModeHTTP && !cfg.BlockUDP {
			return fmt.Errorf("http proxy cannot route udp; BlockUDP must be true")
		}
		return nil
	default:
		return fmt.Errorf("unsupported isolation mode: %s", cfg.Mode)
	}
}
