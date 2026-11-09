package task

import "testing"

func TestParseAPKBadging(t *testing.T) {
	output := "package: name='com.haoyisheng.mobile.app' versionCode='60437' versionName='6.4.37' compileSdkVersion='35'\n" +
		"sdkVersion:'23'\n"

	meta, err := ParseAPKBadging(output)
	if err != nil {
		t.Fatalf("解析 APK 元数据失败: %v", err)
	}
	if meta.PackageName != "com.haoyisheng.mobile.app" || meta.VersionCode != "60437" || meta.VersionName != "6.4.37" {
		t.Fatalf("APK 元数据不匹配: %+v", meta)
	}
	if meta.MinSDK != "23" {
		t.Fatalf("最低 SDK 解析错误: %s", meta.MinSDK)
	}
}

func TestParseAPKBadgingRejectsMissingIdentity(t *testing.T) {
	if _, err := ParseAPKBadging("application-label:'好医生'"); err == nil {
		t.Fatal("缺少 package/versionCode 时必须拒绝")
	}
}
