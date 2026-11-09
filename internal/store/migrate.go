package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// MaxSupportedSchemaVersion 当前代码支持的最大数据库版本
const MaxSupportedSchemaVersion = 2

// ErrDatabaseVersionTooHigh 当数据库版本高于当前程序时返回拒绝错误
var ErrDatabaseVersionTooHigh = errors.New("DATABASE_VERSION_TOO_HIGH: 数据库版本高于程序支持的版本，拒绝写入")

// GetCurrentVersion 获取当前数据库已应用的版本号
func GetCurrentVersion(db *sql.DB) (int, error) {
	// 检查 schema_migrations 表是否存在
	var tableName string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&tableName)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	var version int
	queryErr := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	if queryErr != nil {
		return 0, queryErr
	}
	return version, nil
}

// Migrate 执行数据库版本向上迁移
func Migrate(db *sql.DB) error {
	currentVersion, err := GetCurrentVersion(db)
	if err != nil {
		return fmt.Errorf("读取数据库版本失败: %w", err)
	}

	// 严禁向前兼容版本过高的数据库（防覆盖高版本库）
	if currentVersion > MaxSupportedSchemaVersion {
		return fmt.Errorf("%w: 数据库版本为 %d, 程序仅支持最高版本 %d", ErrDatabaseVersionTooHigh, currentVersion, MaxSupportedSchemaVersion)
	}

	// 版本一致，无需迁移
	if currentVersion == MaxSupportedSchemaVersion {
		return nil
	}

	for version := currentVersion + 1; version <= MaxSupportedSchemaVersion; version++ {
		pattern := fmt.Sprintf("migrations/%03d_*.sql", version)
		matches, matchErr := migrationFS.ReadDir("migrations")
		if matchErr != nil {
			return fmt.Errorf("读取迁移目录失败: %w", matchErr)
		}
		var candidates []string
		for _, entry := range matches {
			matched, pathErr := filepath.Match(strings.TrimPrefix(pattern, "migrations/"), entry.Name())
			if pathErr != nil {
				return fmt.Errorf("解析迁移文件模式失败: %w", pathErr)
			}
			if matched {
				candidates = append(candidates, "migrations/"+entry.Name())
			}
		}
		sort.Strings(candidates)
		if len(candidates) != 1 {
			return fmt.Errorf("数据库版本 %d 必须且只能有一个迁移文件，实际为 %d", version, len(candidates))
		}
		sqlBytes, readErr := migrationFS.ReadFile(candidates[0])
		if readErr != nil {
			return fmt.Errorf("读取迁移文件失败: %w", readErr)
		}

		tx, txErr := db.Begin()
		if txErr != nil {
			return fmt.Errorf("启动迁移事务失败: %w", txErr)
		}
		defer tx.Rollback()

		if _, execErr := tx.Exec(string(sqlBytes)); execErr != nil {
			return fmt.Errorf("执行 %s 失败: %w", candidates[0], execErr)
		}

		now := time.Now().UTC()
		_, recordErr := tx.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", version, now)
		if recordErr != nil {
			return fmt.Errorf("记录迁移版本失败: %w", recordErr)
		}

		if commitErr := tx.Commit(); commitErr != nil {
			return fmt.Errorf("提交迁移事务失败: %w", commitErr)
		}
	}

	return nil
}
