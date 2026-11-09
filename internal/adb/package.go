package adb

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/execx"
)

var (
	installedVersionCodePattern = regexp.MustCompile(`(?m)^\s*versionCode=(\S+)`)
	installedVersionNamePattern = regexp.MustCompile(`(?m)^\s*versionName=(\S*)`)
)

// PackageInfo is the package identity reported by the device package manager.
type PackageInfo struct {
	Installed   bool   `json:"installed"`
	PackageName string `json:"package_name"`
	VersionCode string `json:"version_code"`
	VersionName string `json:"version_name"`
}

// ParsePackageInfo extracts version fields from `dumpsys package` output.
func ParsePackageInfo(packageName, output string) (*PackageInfo, error) {
	versionCode := installedVersionCodePattern.FindStringSubmatch(output)
	if len(versionCode) != 2 {
		return nil, fmt.Errorf("包管理器未返回 %s 的 versionCode", packageName)
	}
	info := &PackageInfo{Installed: true, PackageName: packageName, VersionCode: versionCode[1]}
	if versionName := installedVersionNamePattern.FindStringSubmatch(output); len(versionName) == 2 {
		info.VersionName = versionName[1]
	}
	return info, nil
}

// GetPackageInfo confirms installation and reads the current version from one explicit device.
func (c *Client) GetPackageInfo(ctx context.Context, serial, packageName string, timeout time.Duration) (*PackageInfo, error) {
	if strings.TrimSpace(packageName) == "" {
		return nil, domain.NewPlatformError(domain.ErrCodeInvalidArgument, "Android 包名不能为空", false)
	}
	pathResult, err := c.RunShell(ctx, serial, []string{"pm", "path", packageName}, timeout)
	if err != nil {
		return nil, err
	}
	installed, err := packageInstalledFromPathResult(packageName, pathResult)
	if err != nil {
		return nil, err
	}
	if !installed {
		return &PackageInfo{PackageName: packageName}, nil
	}
	dumpResult, err := c.RunShell(ctx, serial, []string{"dumpsys", "package", packageName}, timeout)
	if err != nil {
		return nil, err
	}
	if dumpResult.ExitCode != 0 {
		return nil, domain.NewPlatformError(domain.ErrCodeProcessExit, fmt.Sprintf("读取应用版本失败（退出码 %d）", dumpResult.ExitCode), true)
	}
	return ParsePackageInfo(packageName, string(dumpResult.Stdout))
}

func packageInstalledFromPathResult(packageName string, result *execx.Result) (bool, error) {
	if result == nil {
		return false, domain.NewPlatformError(domain.ErrCodeProcessExit, "包管理器未返回查询结果", true)
	}
	if result.ExitCode != 0 {
		return false, domain.NewPlatformError(
			domain.ErrCodeProcessExit,
			fmt.Sprintf("查询 %s 安装路径失败（退出码 %d）", packageName, result.ExitCode),
			true,
		)
	}
	return strings.Contains(string(result.Stdout), "package:"), nil
}

// PackageLaunchConfirmed accepts only Activity Manager's explicit success status.
func PackageLaunchConfirmed(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "Status: ok" {
			return true
		}
	}
	return false
}

// StartPackage launches the package's declared launcher activity without hard-coding an Activity class.
func (c *Client) StartPackage(ctx context.Context, serial, packageName string, timeout time.Duration) error {
	result, err := c.RunShell(ctx, serial, []string{
		"am", "start", "-W", "-a", "android.intent.action.MAIN",
		"-c", "android.intent.category.LAUNCHER", "-p", packageName,
	}, timeout)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 || !PackageLaunchConfirmed(string(result.Stdout)) {
		return domain.NewPlatformError(domain.ErrCodeProcessExit, "Android 未确认好医生启动成功", true)
	}
	return nil
}

// ForceStopPackage stops only the named app and never stops the emulator instance.
func (c *Client) ForceStopPackage(ctx context.Context, serial, packageName string, timeout time.Duration) error {
	result, err := c.RunShell(ctx, serial, []string{"am", "force-stop", packageName}, timeout)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return domain.NewPlatformError(domain.ErrCodeProcessExit, fmt.Sprintf("停止好医生失败（退出码 %d）", result.ExitCode), true)
	}
	return nil
}
