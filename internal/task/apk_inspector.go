package task

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
	packageNamePattern = regexp.MustCompile(`\bname='([^']+)'`)
	versionCodePattern = regexp.MustCompile(`\bversionCode='([^']+)'`)
	versionNamePattern = regexp.MustCompile(`\bversionName='([^']*)'`)
	minSDKPattern      = regexp.MustCompile(`(?m)^sdkVersion:'([^']+)'`)
)

// APKIdentity is the signed manifest identity reported by Android build tools.
type APKIdentity struct {
	PackageName string `json:"package_name"`
	VersionCode string `json:"version_code"`
	VersionName string `json:"version_name"`
	MinSDK      string `json:"min_sdk"`
}

// ParseAPKBadging parses the stable identity fields from `aapt2 dump badging`.
func ParseAPKBadging(output string) (*APKIdentity, error) {
	var packageLine string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "package:") {
			packageLine = line
			break
		}
	}
	if packageLine == "" {
		return nil, fmt.Errorf("APK 元数据缺少 package 声明")
	}

	name := packageNamePattern.FindStringSubmatch(packageLine)
	versionCode := versionCodePattern.FindStringSubmatch(packageLine)
	if len(name) != 2 || strings.TrimSpace(name[1]) == "" || len(versionCode) != 2 || strings.TrimSpace(versionCode[1]) == "" {
		return nil, fmt.Errorf("APK 元数据缺少包名或 versionCode")
	}
	identity := &APKIdentity{PackageName: name[1], VersionCode: versionCode[1]}
	if versionName := versionNamePattern.FindStringSubmatch(packageLine); len(versionName) == 2 {
		identity.VersionName = versionName[1]
	}
	if minSDK := minSDKPattern.FindStringSubmatch(output); len(minSDK) == 2 {
		identity.MinSDK = minSDK[1]
	}
	return identity, nil
}

// InspectAPKIdentity invokes the native Android SDK tool with a fixed argument array.
func InspectAPKIdentity(ctx context.Context, executable, path string, timeout time.Duration) (*APKIdentity, error) {
	if strings.TrimSpace(executable) == "" {
		return nil, domain.NewPlatformError(domain.ErrCodeToolMissing, "未配置 aapt2 可执行路径", false)
	}
	result, err := execx.Run(ctx, executable, []string{"dump", "badging", path}, timeout, 512*1024)
	if err != nil {
		return nil, fmt.Errorf("读取 APK 元数据: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, domain.NewPlatformError(domain.ErrCodeProcessExit, fmt.Sprintf("aapt2 解析 APK 失败（退出码 %d）", result.ExitCode), false)
	}
	return ParseAPKBadging(string(result.Stdout))
}
