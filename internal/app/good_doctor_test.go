package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/task"
)

func TestPrepareGoodDoctorAPKInspectsStagedBytes(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "doctor.apk")
	content := append([]byte("PK\x03\x04"), make([]byte, 2048)...)
	if err := os.WriteFile(original, content, 0600); err != nil {
		t.Fatalf("创建测试 APK: %v", err)
	}
	var inspectedPath string
	staged, identity, err := prepareGoodDoctorAPK(
		context.Background(), original, dir, "aapt2", time.Second,
		func(_ context.Context, _, path string, _ time.Duration) (*task.APKIdentity, error) {
			inspectedPath = path
			if err := os.WriteFile(original, []byte("replaced"), 0600); err != nil {
				t.Fatalf("替换原文件: %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil || len(got) != len(content) {
				t.Fatalf("检查器必须读取 staged 文件: len=%d err=%v", len(got), err)
			}
			return &task.APKIdentity{PackageName: GoodDoctorPackageName, VersionCode: "60437"}, nil
		},
	)
	if err != nil {
		t.Fatalf("准备 APK 失败: %v", err)
	}
	defer os.Remove(staged)
	if inspectedPath == original || inspectedPath != staged || identity.VersionCode != "60437" {
		t.Fatalf("必须只检查 staged 文件: original=%s staged=%s inspected=%s", original, staged, inspectedPath)
	}
}

func TestNormalizeLoginUpdate(t *testing.T) {
	status, label, err := normalizeLoginUpdate(GoodDoctorLoginRequest{
		Status: domain.LoginStatusLoggedIn, PhoneLast4: "1234",
	})
	if err != nil {
		t.Fatalf("合法登录状态被拒绝: %v", err)
	}
	if status != domain.LoginStatusLoggedIn || label != "****1234" {
		t.Fatalf("归一化结果错误: %s %s", status, label)
	}
}

func TestNormalizeLoginUpdateRejectsSensitiveInput(t *testing.T) {
	for _, last4 := range []string{"13800138000", "12a4", "123"} {
		if _, _, err := normalizeLoginUpdate(GoodDoctorLoginRequest{
			Status: domain.LoginStatusLoggedIn, PhoneLast4: last4,
		}); err == nil {
			t.Fatalf("敏感或非法号码输入必须拒绝: %q", last4)
		}
	}
}

func TestNormalizeLoginUpdateRejectsSystemStatus(t *testing.T) {
	if _, _, err := normalizeLoginUpdate(GoodDoctorLoginRequest{
		Status: domain.LoginStatusNotInstalled,
	}); err == nil {
		t.Fatal("用户不得手工写入 not_installed")
	}
}
