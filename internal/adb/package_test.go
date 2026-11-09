package adb

import (
	"testing"

	"cloudmatrix/internal/execx"
)

func TestParsePackageInfo(t *testing.T) {
	output := `Packages:
  Package [com.haoyisheng.mobile.app] (123):
    versionCode=60437 minSdk=23 targetSdk=35
    versionName=6.4.37
`
	info, err := ParsePackageInfo("com.haoyisheng.mobile.app", output)
	if err != nil {
		t.Fatalf("解析安装包信息失败: %v", err)
	}
	if info.PackageName != "com.haoyisheng.mobile.app" || info.VersionCode != "60437" || info.VersionName != "6.4.37" {
		t.Fatalf("安装包信息不匹配: %+v", info)
	}
}

func TestPackagePathNonZeroIsAnError(t *testing.T) {
	installed, err := packageInstalledFromPathResult("com.haoyisheng.mobile.app", &execx.Result{
		ExitCode: 1,
		Stderr:   []byte("package manager unavailable"),
	})
	if err == nil || installed {
		t.Fatalf("包管理器异常不得被视为未安装: installed=%v err=%v", installed, err)
	}
}

func TestPackageLaunchConfirmed(t *testing.T) {
	if !PackageLaunchConfirmed("Status: ok\nActivity: com.haoyisheng.mobile.app/.MainActivity\n") {
		t.Fatal("Status: ok 应视为启动确认")
	}
	if PackageLaunchConfirmed("Error: Activity not started") {
		t.Fatal("错误输出不得视为启动成功")
	}
}
