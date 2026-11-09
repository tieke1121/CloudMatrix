package task

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/execx"
)

var (
	// ErrUnsupportedPackageType 不支持的拆分包或复合包格式
	ErrUnsupportedPackageType = errors.New("UNSUPPORTED_PACKAGE_TYPE: 首期仅支持标准单个 .apk 文件，不支持 .xapk/.apks 拆分包")
	// ErrInvalidAPKFile 非法或损坏的 APK 文件
	ErrInvalidAPKFile = errors.New("INVALID_APK_FILE: 文件不是有效的 APK 安装包")
	// ErrAPKFileTooLarge APK 文件体积超出配置限制
	ErrAPKFileTooLarge = errors.New("APK_FILE_TOO_LARGE: APK 文件超出最大允许体积")
)

// APKMeta APK 文件校验结果元数据
type APKMeta struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// ValidateAPKFile 严格校验本地 APK 文件，包含类型、魔数与哈希摘要
func ValidateAPKFile(path string, maxSize int64) (*APKMeta, error) {
	if strings.TrimSpace(path) == "" {
		return nil, domain.NewPlatformError(domain.ErrCodeInvalidArgument, "APK 路径不能为空", false)
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".xapk" || ext == ".apks" {
		return nil, ErrUnsupportedPackageType
	}
	if ext != ".apk" {
		return nil, fmt.Errorf("%w: 不支持的扩展名 %s", ErrInvalidAPKFile, ext)
	}

	fi, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("APK 文件不存在或无法访问: %w", err)
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New("APK 必须是普通文件")
	}

	size := fi.Size()
	if size < 1024 {
		return nil, fmt.Errorf("%w: 文件过小 (<1KB)", ErrInvalidAPKFile)
	}
	if maxSize > 0 && size > maxSize {
		return nil, fmt.Errorf("%w: 当前大小 %d 字节，上限 %d 字节", ErrAPKFileTooLarge, size, maxSize)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// 校验 Zip 归档魔数 (PK\x03\x04)
	magic := make([]byte, 4)
	if _, readErr := io.ReadFull(f, magic); readErr != nil {
		return nil, fmt.Errorf("读取文件头失败: %w", readErr)
	}
	if string(magic) != "PK\x03\x04" {
		return nil, fmt.Errorf("%w: 文件缺少有效的 Zip/APK 魔数签名", ErrInvalidAPKFile)
	}

	// 计算 SHA-256 哈希值
	if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
		return nil, seekErr
	}
	hasher := sha256.New()
	if copied, copyErr := io.Copy(hasher, io.LimitReader(f, size+1)); copyErr != nil {
		return nil, fmt.Errorf("计算 APK 哈希失败: %w", copyErr)
	} else if copied != size {
		return nil, fmt.Errorf("APK 文件在校验期间变化")
	}
	hashStr := hex.EncodeToString(hasher.Sum(nil))

	return &APKMeta{
		Path:   path,
		Size:   size,
		SHA256: hashStr,
	}, nil
}

// StageAPK copies exactly the validated bytes to an app-owned temporary file.
// A queued job must never reopen the user's original path.
func StageAPK(meta *APKMeta, dataDir string) (string, error) {
	input, err := os.Open(meta.Path)
	if err != nil {
		return "", err
	}
	defer input.Close()
	output, err := os.CreateTemp(dataDir, "install-*.apk")
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(output, hash), io.LimitReader(input, meta.Size+1))
	closeErr := output.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return "", errors.Join(err, os.Remove(output.Name()))
	}
	if size != meta.Size || hex.EncodeToString(hash.Sum(nil)) != meta.SHA256 {
		return "", errors.Join(errors.New("APK 在校验后已变化"), os.Remove(output.Name()))
	}
	return output.Name(), nil
}

// ParseADBInstallError 解析 ADB 安装命令返回的标准错误信息
func ParseADBInstallError(stdout string, stderr string) (code string, message string) {
	combined := stdout + "\n" + stderr
	switch {
	case strings.Contains(combined, "INSTALL_FAILED_ALREADY_EXISTS"):
		return "INSTALL_FAILED_ALREADY_EXISTS", "应用已存在且未指定覆盖安装"
	case strings.Contains(combined, "INSTALL_FAILED_INSUFFICIENT_STORAGE"):
		return "INSTALL_FAILED_INSUFFICIENT_STORAGE", "设备存储空间不足"
	case strings.Contains(combined, "INSTALL_FAILED_INVALID_APK"):
		return "INSTALL_FAILED_INVALID_APK", "无效或解析损坏的安装包"
	case strings.Contains(combined, "INSTALL_FAILED_OLDER_SDK"):
		return "INSTALL_FAILED_OLDER_SDK", "应用要求的最低 SDK 高于当前 Android 系统版本"
	default:
		return "INSTALL_FAILED", strings.TrimSpace(combined)
	}
}

type apkInstaller interface {
	Install(context.Context, string, string, time.Duration) (*execx.Result, error)
}

// InstallAPKOnDevice 在指定 ADB 设备上执行安装
func InstallAPKOnDevice(ctx context.Context, client apkInstaller, serial string, apkPath string, timeout time.Duration) error {
	if client == nil {
		return errors.New("ADB 客户端未初始化")
	}

	res, err := client.Install(ctx, serial, apkPath, timeout)
	if err != nil {
		return err
	}
	if res == nil {
		return errors.New("ADB 未返回安装结果")
	}
	for _, line := range strings.Split(string(res.Stdout), "\n") {
		if res.ExitCode == 0 && strings.TrimSpace(line) == "Success" {
			return nil
		}
	}
	code, message := ParseADBInstallError(string(res.Stdout), string(res.Stderr))
	if code == "INSTALL_FAILED" {
		message = fmt.Sprintf("ADB 未确认安装成功（退出码 %d）", res.ExitCode)
	}
	return domain.NewPlatformError(code, message, false)
}
