package main

import (
	"context"
	"strings"
	"testing"

	"cloudmatrix/internal/config"
)

// 测试 App 结构体初始化与默认状态输出
func TestAppSystemStatus(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	app.startup(context.Background())
	t.Cleanup(app.shutdown)

	status := app.GetSystemStatus()
	if !strings.Contains(status, "CloudMatrix") {
		t.Fatalf("预期状态包含 CloudMatrix，实际为: %s", status)
	}

	greeting := app.Greet("Developer")
	if !strings.Contains(greeting, "Developer") {
		t.Fatalf("预期包含问候名称 Developer，实际为: %s", greeting)
	}
}

func TestAppRefusesConcurrentUseOfSameDataDirectory(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	app, err := NewApp(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.shutdown)
	if second, err := NewApp(cfg); err == nil {
		second.shutdown()
		t.Fatal("second desktop must not interrupt the first desktop's tasks")
	}
}
