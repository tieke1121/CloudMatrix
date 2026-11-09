package stream

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cloudmatrix/internal/domain"
)

// 获取测试用可执行文件夹具
func getTestChildFixture(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取目录失败: %v", err)
	}
	target := filepath.Join(wd, "..", "..", "testdata", "child", "child.exe")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("测试夹具未找到: %s", target)
	}
	return target
}

// 1. 测试 scrcpy 参数切片构造
func TestBuildScrcpyArgs(t *testing.T) {
	opts := ScrcpyOptions{
		Serial:      "127.0.0.1:16384",
		WindowTitle: "Test Window",
		NoAudio:     true,
		StayAwake:   true,
		MaxFPS:      30,
		MaxSize:     1080,
	}

	args, err := BuildScrcpyArgs(opts)
	if err != nil {
		t.Fatalf("构造参数失败: %v", err)
	}

	argStr := ""
	for _, a := range args {
		argStr += a + " "
	}

	if args[0] != "--serial" || args[1] != "127.0.0.1:16384" {
		t.Errorf("serial 参数不符: %v", args)
	}
	if !contains(args, "--no-audio") {
		t.Errorf("缺少 --no-audio 参数: %s", argStr)
	}
	if !contains(args, "--stay-awake") {
		t.Errorf("缺少 --stay-awake 参数: %s", argStr)
	}
	if !contains(args, "--max-fps=30") {
		t.Errorf("缺少 --max-fps=30: %s", argStr)
	}

	// 空 serial 必须被拒绝
	_, emptyErr := BuildScrcpyArgs(ScrcpyOptions{})
	if emptyErr == nil {
		t.Fatal("空 serial 应报错拒绝")
	}
}

func contains(arr []string, val string) bool {
	for _, s := range arr {
		if s == val {
			return true
		}
	}
	return false
}

// 2. 测试投屏生命周期与单例会话保证（重复打开返回同一会话，V06 验收）
func TestStreamManagerLifecycle(t *testing.T) {
	fixture := getTestChildFixture(t)
	mgr := NewStreamManager(fixture)

	ctx := context.Background()
	opts := ScrcpyOptions{
		Serial: "127.0.0.1:16384",
	}

	// 首次打开投屏
	s1, err := mgr.StartSession(ctx, "inst-001", opts)
	if err != nil {
		t.Fatalf("首次启动会话失败: %v", err)
	}
	if !mgr.HasSession("inst-001") {
		t.Fatal("HasSession 应返回 true")
	}

	// 再次打开同一实例投屏，必须返回同一会话句柄（幂等单例）
	s2, err := mgr.StartSession(ctx, "inst-001", opts)
	if err != nil {
		t.Fatalf("再次打开会话失败: %v", err)
	}
	if s1 != s2 {
		t.Fatal("预期返回同一会话句柄，实际创建了新会话")
	}

	// 主动停止会话
	stopErr := mgr.StopSession("inst-001")
	if stopErr != nil {
		t.Fatalf("停止会话失败: %v", stopErr)
	}

	if mgr.HasSession("inst-001") {
		t.Fatal("停止后 HasSession 应返回 false")
	}
}

// 3. 测试投屏子进程自发退出后的自动回收（V06 验收核心：关闭投屏窗口不关闭实例）
func TestStreamManagerChildExitCleanup(t *testing.T) {
	fixture := getTestChildFixture(t)
	mgr := NewStreamManager(fixture)

	ctx := context.Background()
	// 使用快速退出的模式测试
	opts := ScrcpyOptions{
		Serial: "127.0.0.1:16384",
	}

	session, err := mgr.StartSession(ctx, "inst-auto-exit", opts)
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	// 主动终止其底层的模拟进程
	_ = session.cmd.Process.Kill()

	// 等待回收
	select {
	case <-session.done:
	case <-time.After(2 * time.Second):
		t.Fatal("子进程退出后未在 2 秒内释放会话")
	}

	time.Sleep(50 * time.Millisecond)
	if mgr.HasSession("inst-auto-exit") {
		t.Fatal("子进程退出后管理器应自动清理该会话记录")
	}
}

// 4. 测试未配置或不存在的工具路径拦截
func TestToolMissingRejection(t *testing.T) {
	mgr := NewStreamManager("D:\\non_existent_scrcpy.exe")
	ctx := context.Background()
	_, err := mgr.StartSession(ctx, "inst-x", ScrcpyOptions{Serial: "127.0.0.1:16384"})
	if err == nil {
		t.Fatal("工具不存在应报错拒绝")
	}

	platErr, ok := err.(*domain.PlatformError)
	if !ok || platErr.Code != domain.ErrCodeToolMissing {
		t.Fatalf("预期返回 TOOL_MISSING 错误，实际为: %v", err)
	}
}
