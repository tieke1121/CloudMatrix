package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// GenerateBackupPath 根据原始数据库路径生成带时间戳的备份文件路径
func GenerateBackupPath(dbPath string) string {
	timestamp := time.Now().Format("20060102-150405")
	dir := filepath.Dir(dbPath)
	base := filepath.Base(dbPath)
	return filepath.Join(dir, fmt.Sprintf("%s.backup.%s", base, timestamp))
}

// BackupDatabase 对现有 SQLite 数据库文件执行安全冷备份拷贝
func BackupDatabase(srcPath, dstPath string) error {
	if err := requireColdDatabase(srcPath); err != nil {
		return err
	}
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source db for backup failed: %w", err)
	}
	defer srcFile.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("create backup directory failed: %w", err)
	}

	dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create backup target file failed: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy database content failed: %w", err)
	}

	return dstFile.Sync()
}

// RestoreDatabase 从指定备份快照文件还原数据库
func RestoreDatabase(backupPath, targetPath string) error {
	if err := requireColdDatabase(targetPath); err != nil {
		return err
	}

	// 采用原子替换方式，先拷贝到临时文件再覆盖目标
	tempTarget := targetPath + ".restore_tmp"
	if err := BackupDatabase(backupPath, tempTarget); err != nil {
		return fmt.Errorf("prepare restored file failed: %w", err)
	}

	// Never delete the current database if replacement fails.
	if err := os.Rename(tempTarget, targetPath); err != nil {
		return fmt.Errorf("替换数据库失败，原文件保留；恢复副本位于 %s: %w", tempTarget, err)
	}

	return nil
}

func requireColdDatabase(path string) error {
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		_, err := os.Lstat(path + suffix)
		if err == nil {
			return fmt.Errorf("数据库存在 %s 文件，请关闭应用并完成 SQLite 检查点后再操作", suffix)
		}
		if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
