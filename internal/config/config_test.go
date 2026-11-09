package config

import (
	"strings"
	"testing"
	"time"
)

// 测试默认配置有效性
func TestDefaultConfigValid(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("默认配置应该合法，实际返回: %v", err)
	}
}

// 测试配置非法范围校验拦截
func TestConfigValidationRanges(t *testing.T) {
	// 1. 负数并发
	cfg := DefaultConfig()
	cfg.Limits.StartConcurrency = -1
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "limits.start_concurrency") {
		t.Fatalf("应拒绝负数启动并发，实际返回: %v", err)
	}

	// 2. 超大任务并发
	cfg = DefaultConfig()
	cfg.Limits.TaskConcurrency = 999
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "limits.task_concurrency") {
		t.Fatalf("应拒绝超出上限的任务并发，实际返回: %v", err)
	}

	// 3. 超大 Shell 超时时间
	cfg = DefaultConfig()
	cfg.Timeouts.MaxShellTimeout = 3600 * time.Second
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "timeouts.max_shell_timeout") {
		t.Fatalf("应拒绝超过 1800 秒的 Shell 超时配置，实际返回: %v", err)
	}

	// 4. 空配置项拦截
	cfg = DefaultConfig()
	cfg.SchemaVersion = ""
	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("应拒绝空 schema_version，实际返回: %v", err)
	}
}
