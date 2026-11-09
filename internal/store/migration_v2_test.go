package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestMigrateVersionOneDatabaseToAppSessions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "v1.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()
	versionOneSQL, err := migrationFS.ReadFile("migrations/001_init.sql")
	if err != nil {
		t.Fatalf("读取 v1 迁移失败: %v", err)
	}
	if _, err := db.Exec(string(versionOneSQL)); err != nil {
		t.Fatalf("创建 v1 schema 失败: %v", err)
	}
	if _, err := db.Exec("INSERT INTO schema_migrations(version, applied_at) VALUES(1, ?)", time.Now().UTC()); err != nil {
		t.Fatalf("记录 v1 版本失败: %v", err)
	}

	if err := Migrate(db); err != nil {
		t.Fatalf("迁移到 v2 失败: %v", err)
	}
	version, err := GetCurrentVersion(db)
	if err != nil || version != 2 {
		t.Fatalf("数据库版本应为 2，实际 %d，错误 %v", version, err)
	}
	var tableName string
	if err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='app_sessions'").Scan(&tableName); err != nil {
		t.Fatalf("app_sessions 表不存在: %v", err)
	}
}
