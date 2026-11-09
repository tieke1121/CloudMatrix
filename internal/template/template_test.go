package template

import (
	"context"
	"errors"
	"testing"
)

// 1. 验证模板基底引用计数保护（引用大于 0 时禁止删除，V12 核心验收）
func TestTemplateRefCountProtection(t *testing.T) {
	mgr := NewTemplateManager(10)
	ctx := context.Background()

	// 创建基底模板
	tpl, err := mgr.CreateTemplate(ctx, "Gold-Android-12", "v1.0", "sha256:abc123456")
	if err != nil {
		t.Fatalf("创建模板失败: %v", err)
	}

	// 从模板克隆实例（磁盘充足 50GB）
	_, cloneErr := mgr.CloneInstance(ctx, tpl.ID, "Instance-Clone-1", 50)
	if cloneErr != nil {
		t.Fatalf("克隆实例失败: %v", cloneErr)
	}

	// 关键断言：此时 RefCount 应为 1，尝试删除模板必须被拦截
	delErr := mgr.DeleteTemplate(tpl.ID)
	if !errors.Is(delErr, ErrTemplateInUse) {
		t.Fatalf("预期返回 ErrTemplateInUse，实际为: %v", delErr)
	}

	// 释放实例引用
	_ = mgr.ReleaseInstance(tpl.ID)

	// RefCount 归零后，删除成功
	delSuccessErr := mgr.DeleteTemplate(tpl.ID)
	if delSuccessErr != nil {
		t.Fatalf("引用清零后删除模板失败: %v", delSuccessErr)
	}
}

// 2. 验证磁盘空间不足时克隆准入拦截
func TestCloneInsufficientDiskProtection(t *testing.T) {
	mgr := NewTemplateManager(15) // 最低要求 15GB 磁盘
	ctx := context.Background()

	tpl, _ := mgr.CreateTemplate(ctx, "Gold-Base", "v1.0", "sha256:xxx")

	// 模拟当前磁盘仅剩 8GB，低于 15GB 门槛
	_, err := mgr.CloneInstance(ctx, tpl.ID, "Inst-New", 8)
	if !errors.Is(err, ErrInsufficientDisk) {
		t.Fatalf("预期返回 ErrInsufficientDisk，实际为: %v", err)
	}
}
