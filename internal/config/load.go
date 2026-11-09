package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Load reads an optional explicit JSON config over the defaults.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	if path != "" {
		path, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("解析 CLOUDMATRIX_CONFIG 路径: %w", err)
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("打开 CLOUDMATRIX_CONFIG: %w", err)
		}
		defer file.Close()
		decoder := json.NewDecoder(file)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("解析 CLOUDMATRIX_CONFIG: %w", err)
		}
		if cfg == nil || strings.TrimSpace(cfg.DataDir) == "" {
			return nil, errors.New("配置必须是 JSON 对象，且 data_dir 不能为空")
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			return nil, errors.New("配置只接受一个 JSON 对象")
		}
		if !filepath.IsAbs(cfg.DataDir) {
			cfg.DataDir = filepath.Join(filepath.Dir(path), cfg.DataDir)
		}
	} else if !filepath.IsAbs(cfg.DataDir) {
		return nil, errors.New("无法定位用户数据目录，请通过 CLOUDMATRIX_CONFIG 指定 data_dir")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// VerifyProviderCLI checks the configured binary matches the operator's verified version.
func (c ProviderConfig) VerifyProviderCLI() error {
	if c.Type != "mumu" {
		return fmt.Errorf("不支持的模拟器适配器 %q", c.Type)
	}
	if c.Executable == "" {
		return nil
	}
	expected, err := hex.DecodeString(c.VerifiedCLISHA256)
	if !filepath.IsAbs(c.Executable) || err != nil || len(expected) != sha256.Size {
		return errors.New("MuMu CLI 需填写已验证的绝对路径和 verified_cli_sha256")
	}
	info, err := os.Lstat(c.Executable)
	if err != nil {
		return fmt.Errorf("读取 MuMu CLI 路径: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("MuMu CLI 路径必须指向普通文件")
	}
	file, err := os.Open(c.Executable)
	if err != nil {
		return fmt.Errorf("打开 MuMu CLI: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("读取 MuMu CLI 摘要: %w", err)
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), c.VerifiedCLISHA256) {
		return errors.New("MuMu CLI 与已验证版本的 SHA256 不匹配")
	}
	return nil
}
