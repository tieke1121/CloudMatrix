package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProviderConfig 厂商工具配置
type ProviderConfig struct {
	Type              string `json:"type"`
	Executable        string `json:"executable"`
	VerifiedCLISHA256 string `json:"verified_cli_sha256"`
}

// ToolConfig 工具路径配置
type ToolConfig struct {
	Executable string `json:"executable"`
}

// LimitsConfig 并发与容量上限配置
type LimitsConfig struct {
	StartConcurrency    int `json:"start_concurrency"`
	TaskConcurrency     int `json:"task_concurrency"`
	MaxRunningInstances int `json:"max_running_instances"`
}

// TimeoutsConfig 关键超时时间配置
type TimeoutsConfig struct {
	QueryTimeout      time.Duration `json:"query_timeout"`
	StartReadyTimeout time.Duration `json:"start_ready_timeout"`
	StopTimeout       time.Duration `json:"stop_timeout"`
	ApkInstallTimeout time.Duration `json:"apk_install_timeout"`
	ShellTimeout      time.Duration `json:"shell_timeout"`
	MaxShellTimeout   time.Duration `json:"max_shell_timeout"`
}

// RetentionConfig 数据保留策略
type RetentionConfig struct {
	MaxEvents int `json:"max_events"`
	MaxTasks  int `json:"max_tasks"`
}

// Config 核心系统配置
type Config struct {
	SchemaVersion string          `json:"schema_version"`
	DataDir       string          `json:"data_dir"`
	Provider      ProviderConfig  `json:"provider"`
	Adb           ToolConfig      `json:"adb"`
	Aapt2         ToolConfig      `json:"aapt2"`
	Scrcpy        ToolConfig      `json:"scrcpy"`
	Limits        LimitsConfig    `json:"limits"`
	Timeouts      TimeoutsConfig  `json:"timeouts"`
	Retention     RetentionConfig `json:"retention"`
}

// DefaultConfig 获取默认配置
func DefaultConfig() *Config {
	dataDir := "data"
	if dir, err := os.UserConfigDir(); err == nil {
		dataDir = filepath.Join(dir, "CloudMatrix", "data")
	}
	return &Config{
		SchemaVersion: "1.0",
		DataDir:       dataDir,
		Provider: ProviderConfig{
			Type:       "mumu",
			Executable: "",
		},
		Adb: ToolConfig{
			Executable: "adb",
		},
		Aapt2: ToolConfig{
			Executable: "aapt2",
		},
		Scrcpy: ToolConfig{
			Executable: "scrcpy",
		},
		Limits: LimitsConfig{
			StartConcurrency:    1,
			TaskConcurrency:     2,
			MaxRunningInstances: 5,
		},
		Timeouts: TimeoutsConfig{
			QueryTimeout:      10 * time.Second,
			StartReadyTimeout: 180 * time.Second,
			StopTimeout:       60 * time.Second,
			ApkInstallTimeout: 180 * time.Second,
			ShellTimeout:      30 * time.Second,
			MaxShellTimeout:   300 * time.Second,
		},
		Retention: RetentionConfig{
			MaxEvents: 10000,
			MaxTasks:  1000,
		},
	}
}

// Validate 严格校验配置合法性与数值范围
func (c *Config) Validate() error {
	if strings.TrimSpace(c.SchemaVersion) == "" {
		return errors.New("配置项 [schema_version] 不能为空")
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return errors.New("配置项 [data_dir] 不能为空")
	}

	// 校验并发限制
	if c.Limits.StartConcurrency < 1 || c.Limits.StartConcurrency > 32 {
		return fmt.Errorf("配置项 [limits.start_concurrency] 范围必须在 1~32 之间，当前为: %d", c.Limits.StartConcurrency)
	}
	if c.Limits.TaskConcurrency < 1 || c.Limits.TaskConcurrency > 64 {
		return fmt.Errorf("配置项 [limits.task_concurrency] 范围必须在 1~64 之间，当前为: %d", c.Limits.TaskConcurrency)
	}
	if c.Limits.MaxRunningInstances < 1 || c.Limits.MaxRunningInstances > 128 {
		return fmt.Errorf("配置项 [limits.max_running_instances] 范围必须在 1~128 之间，当前为: %d", c.Limits.MaxRunningInstances)
	}

	// 校验超时时间限制
	if c.Timeouts.QueryTimeout <= 0 || c.Timeouts.QueryTimeout > 60*time.Second {
		return fmt.Errorf("配置项 [timeouts.query_timeout] 超出合法区间 (1s~60s): %v", c.Timeouts.QueryTimeout)
	}
	if c.Timeouts.StartReadyTimeout < 10*time.Second || c.Timeouts.StartReadyTimeout > 600*time.Second {
		return fmt.Errorf("配置项 [timeouts.start_ready_timeout] 超出合法区间 (10s~600s): %v", c.Timeouts.StartReadyTimeout)
	}
	if c.Timeouts.ApkInstallTimeout <= 0 || c.Timeouts.ApkInstallTimeout > 30*time.Minute {
		return fmt.Errorf("配置项 [timeouts.apk_install_timeout] 必须在 0~30 分钟之间")
	}
	if c.Timeouts.StopTimeout < 5*time.Second || c.Timeouts.StopTimeout > 300*time.Second {
		return fmt.Errorf("配置项 [timeouts.stop_timeout] 超出合法区间 (5s~300s): %v", c.Timeouts.StopTimeout)
	}
	if c.Timeouts.ShellTimeout <= 0 || c.Timeouts.ShellTimeout > c.Timeouts.MaxShellTimeout {
		return fmt.Errorf("配置项 [timeouts.shell_timeout] 必须大于 0 且不超过 max_shell_timeout: %v", c.Timeouts.ShellTimeout)
	}
	if c.Timeouts.MaxShellTimeout > 1800*time.Second {
		return fmt.Errorf("配置项 [timeouts.max_shell_timeout] 不能超过 1800 秒: %v", c.Timeouts.MaxShellTimeout)
	}

	return nil
}
