package task

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// 1. 验证拒绝 .xapk / .apks
func TestValidateAPKRejectsXAPK(t *testing.T) {
	_, err := ValidateAPKFile("test_app.xapk", 100*1024*1024)
	if !errors.Is(err, ErrUnsupportedPackageType) {
		t.Fatalf("预期返回 ErrUnsupportedPackageType，实际返回: %v", err)
	}

	_, apksErr := ValidateAPKFile("test_app.apks", 100*1024*1024)
	if !errors.Is(apksErr, ErrUnsupportedPackageType) {
		t.Fatalf("预期返回 ErrUnsupportedPackageType，实际返回: %v", apksErr)
	}
}

// 2. 验证非 Zip/APK 魔数文件拒绝
func TestValidateAPKRejectsInvalidMagic(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "cm-apk-*")
	defer os.RemoveAll(tempDir)

	fakeAPK := filepath.Join(tempDir, "fake.apk")
	// 写入假数据（非 PK\x03\x04）
	_ = os.WriteFile(fakeAPK, []byte("NOT_A_ZIP_FILE_12345678901234567890"), 0644)

	_, err := ValidateAPKFile(fakeAPK, 100*1024*1024)
	if !errors.Is(err, ErrInvalidAPKFile) {
		t.Fatalf("预期返回 ErrInvalidAPKFile，实际返回: %v", err)
	}
}

// 3. 验证有效 APK（含 Zip 魔数且大于 1KB）计算哈希
func TestValidateAPKValidFile(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "cm-apk-valid-*")
	defer os.RemoveAll(tempDir)

	validAPK := filepath.Join(tempDir, "sample.apk")
	// 构造合法的 Zip 文件头与填充
	header := []byte{'P', 'K', 0x03, 0x04}
	content := append(header, make([]byte, 2048)...)
	_ = os.WriteFile(validAPK, content, 0644)

	meta, err := ValidateAPKFile(validAPK, 10*1024*1024)
	if err != nil {
		t.Fatalf("合法 APK 校验不应报错: %v", err)
	}

	if meta.Size != int64(len(content)) {
		t.Errorf("文件大小不符: %d", meta.Size)
	}
	if len(meta.SHA256) != 64 {
		t.Errorf("SHA256 格式不符: %s", meta.SHA256)
	}
}

// 4. 测试 ADB 安装错误解析
func TestParseADBInstallError(t *testing.T) {
	code, msg := ParseADBInstallError("Failure [INSTALL_FAILED_ALREADY_EXISTS]", "")
	if code != "INSTALL_FAILED_ALREADY_EXISTS" || msg == "" {
		t.Errorf("解析 INSTALL_FAILED_ALREADY_EXISTS 失败: %s, %s", code, msg)
	}

	code2, _ := ParseADBInstallError("", "Failure [INSTALL_FAILED_INSUFFICIENT_STORAGE]")
	if code2 != "INSTALL_FAILED_INSUFFICIENT_STORAGE" {
		t.Errorf("解析 INSTALL_FAILED_INSUFFICIENT_STORAGE 失败: %s", code2)
	}
}
