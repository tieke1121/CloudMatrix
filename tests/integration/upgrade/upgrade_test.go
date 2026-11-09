package upgrade_integration_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/store"
)

// TestUpgradeAndBackupWorkflow 验证升级前自动备份与快照有效性
func TestUpgradeAndBackupWorkflow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cm-upgrade-test-*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()
	dbPath := filepath.Join(tempDir, "cloudmatrix.db")

	// 1. 初始化原始数据库并写入基线数据
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("init store failed: %v", err)
	}

	inst1 := &domain.Instance{
		ID:                 "inst-v1",
		HostID:             "local",
		ProviderType:       "mumu",
		ProviderInstanceID: "0",
		Name:               "MuMu-Baseline",
		DesiredState:       domain.StateStopped,
		ObservedState:      domain.StateStopped,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if _, err := s.UpsertInstance(ctx, inst1); err != nil {
		t.Fatalf("save instance failed: %v", err)
	}
	_ = s.Close()

	// 2. 模拟升级前执行备份
	backupPath := store.GenerateBackupPath(dbPath)
	if err := store.BackupDatabase(dbPath, backupPath); err != nil {
		t.Fatalf("BackupDatabase failed: %v", err)
	}

	// 验证备份文件存在且大小非空
	fi, err := os.Stat(backupPath)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("backup file invalid or empty: %v", err)
	}

	// 3. 模拟升级过程发生意外修改
	s2, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen store failed: %v", err)
	}
	instBad := &domain.Instance{
		ID:                 "inst-corrupted",
		HostID:             "local",
		ProviderType:       "mumu",
		ProviderInstanceID: "1",
		Name:               "Corrupted-Instance",
		DesiredState:       domain.StateError,
		ObservedState:      domain.StateError,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	_, _ = s2.UpsertInstance(ctx, instBad)
	_ = s2.Close()

	// 4. 执行回滚恢复备份
	if err := store.RestoreDatabase(backupPath, dbPath); err != nil {
		t.Fatalf("RestoreDatabase failed: %v", err)
	}

	// 5. 验证还原后的数据库状态与基线完全一致，异常修改被剔除
	sRestored, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("open restored store failed: %v", err)
	}
	defer sRestored.Close()

	instances, err := sRestored.ListInstances(ctx)
	if err != nil {
		t.Fatalf("list instances failed: %v", err)
	}
	if len(instances) != 1 || instances[0].Name != "MuMu-Baseline" {
		t.Fatalf("rollback failed, unexpected instances: %+v", instances)
	}
}

// TestHigherVersionSchemaRejection 验证高版本数据库拒绝低版本程序访问
func TestHigherVersionSchemaRejection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cm-schema-ver-*")
	if err != nil {
		t.Fatalf("create temp dir failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "high_ver.db")

	// 先按正常方式初始化
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("init store failed: %v", err)
	}
	_ = s.Close()

	// 人为伪造写入一个高版本（例如版本 99）
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw sqlite failed: %v", err)
	}
	_, err = rawDB.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (99, CURRENT_TIMESTAMP)")
	_ = rawDB.Close()
	if err != nil {
		t.Fatalf("insert high version failed: %v", err)
	}

	// 尝试以当前程序（仅支持版本 1）重新打开该高版本库：必须报错拒绝！
	_, openErr := store.NewSQLiteStore(dbPath)
	if openErr == nil {
		t.Fatal("expected ErrDatabaseVersionTooHigh for version 99, got nil")
	}
	if !errors.Is(openErr, store.ErrDatabaseVersionTooHigh) {
		t.Fatalf("expected error wrapping ErrDatabaseVersionTooHigh, got: %v", openErr)
	}
}
