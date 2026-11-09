package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cloudmatrix/internal/domain"
)

// 1. 测试实例去重与唯一性更新（V03 核心验收）
func TestUpsertInstanceUniqueness(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cm-store-test-*")
	if err != nil {
		t.Fatalf("创建临时测试目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("初始化 SQLiteStore 失败: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 第一次发现模拟器 0 号实例
	inst1 := &domain.Instance{
		ID:                 "uuid-1",
		HostID:             "local",
		ProviderType:       "mumu",
		ProviderInstanceID: "0",
		Name:               "MuMu-0",
		DesiredState:       domain.StateStopped,
		ObservedState:      domain.StateStopped,
		AdbSerial:          "127.0.0.1:16384",
	}

	saved1, err := store.UpsertInstance(ctx, inst1)
	if err != nil {
		t.Fatalf("首次插入实例失败: %v", err)
	}
	if saved1.ID != "uuid-1" {
		t.Fatalf("预期 ID 为 uuid-1，实际为: %s", saved1.ID)
	}

	// 第二次扫描刷新（生成了新的临时 uuid-2，但底层厂商 ID 相同）
	inst2 := &domain.Instance{
		ID:                 "uuid-2",
		HostID:             "local",
		ProviderType:       "mumu",
		ProviderInstanceID: "0",
		Name:               "MuMu-0-Updated",
		DesiredState:       domain.StateReady,
		ObservedState:      domain.StateReady,
		AdbSerial:          "127.0.0.1:16384",
	}

	saved2, err := store.UpsertInstance(ctx, inst2)
	if err != nil {
		t.Fatalf("更新已有实例失败: %v", err)
	}

	// 关键断言：必须复用最初记录的 uuid-1，绝不能产生新行或主键漂移
	if saved2.ID != "uuid-1" {
		t.Fatalf("预期保持原主键 uuid-1，实际漂移为: %s", saved2.ID)
	}
	if saved2.ObservedState != domain.StateReady {
		t.Fatalf("预期状态更新为 ready，实际为: %s", saved2.ObservedState)
	}

	// 验证总行数仅为 1 条
	list, err := store.ListInstances(ctx)
	if err != nil {
		t.Fatalf("查询实例列表失败: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("预期实例列表长度为 1，实际为: %d", len(list))
	}
}

// 2. 测试任务幂等键防重提交
func TestTaskCreateIdempotency(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cm-store-task-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("初始化失败: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	task1 := &domain.Task{
		ID:             "task-001",
		Type:           "start",
		Status:         domain.TaskQueued,
		IdempotencyKey: "idem-key-999",
		Targets: []*domain.TaskTarget{
			{
				ID:         "target-001",
				InstanceID: "uuid-1",
				Status:     domain.TaskQueued,
			},
		},
	}

	err = store.CreateTask(ctx, task1)
	if err != nil {
		t.Fatalf("首次创建任务失败: %v", err)
	}

	// 重复提交相同幂等键的任务
	task2 := &domain.Task{
		ID:             "task-002",
		Type:           "start",
		Status:         domain.TaskQueued,
		IdempotencyKey: "idem-key-999",
		Targets: []*domain.TaskTarget{
			{
				ID:         "target-002",
				InstanceID: "uuid-1",
				Status:     domain.TaskQueued,
			},
		},
	}

	dupErr := store.CreateTask(ctx, task2)
	if dupErr == nil {
		t.Fatal("预期冲突错误，实际创建成功")
	}

	var platErr *domain.PlatformError
	if !errors.As(dupErr, &platErr) || platErr.Code != domain.ErrCodeConflict {
		t.Fatalf("预期返回 CONFLICT 错误，实际为: %v", dupErr)
	}
}

// 3. 测试高版本数据库拒绝写入（V03 核心验收）
func TestDatabaseVersionHigherRejection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cm-store-ver-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")

	// 模拟已存在一个版本为 999 的高版本数据库
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	_, err = rawDB.Exec("CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at DATETIME);")
	if err != nil {
		t.Fatalf("创建版本表失败: %v", err)
	}
	_, err = rawDB.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (999, ?);", time.Now())
	if err != nil {
		t.Fatalf("写入高版本记录失败: %v", err)
	}
	rawDB.Close()

	// 尝试以当前程序加载该高版本库
	_, openErr := NewSQLiteStore(dbPath)
	if openErr == nil {
		t.Fatal("预期拒绝加载高于程序支持的版本，实际未报错")
	}

	if !strings.Contains(openErr.Error(), "DATABASE_VERSION_TOO_HIGH") {
		t.Fatalf("预期错误信息包含 DATABASE_VERSION_TOO_HIGH，实际为: %v", openErr)
	}
}
