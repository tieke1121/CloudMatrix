package execx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 获取编译好的测试夹具可执行文件路径
func getChildExecutable(t *testing.T) string {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取工作目录失败: %v", err)
	}

	// 尝试寻找 testdata/child/child.exe
	target := filepath.Join(wd, "..", "..", "testdata", "child", "child.exe")
	if _, err := os.Stat(target); err != nil {
		// 若从当前目录查找
		target = filepath.Join(wd, "testdata", "child", "child.exe")
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("测试夹具不存在，请先编译: %s", target)
		}
	}
	return target
}

// 1. 验证常规成功执行
func TestRunSuccess(t *testing.T) {
	child := getChildExecutable(t)
	ctx := context.Background()

	res, err := Run(ctx, child, []string{"exit", "0"}, 5*time.Second, 1024)
	if err != nil {
		t.Fatalf("预期成功，实际返回错误: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("预期退出码 0，实际为: %d", res.ExitCode)
	}
}

// 2. 验证非零退出码（退出码 7）
func TestRunExitCode7(t *testing.T) {
	child := getChildExecutable(t)
	ctx := context.Background()

	res, err := Run(ctx, child, []string{"exit", "7"}, 5*time.Second, 1024)
	if err != nil {
		t.Fatalf("非零退出不应返回 Go 错误，实际返回: %v", err)
	}
	if res.ExitCode != 7 {
		t.Fatalf("预期退出码 7，实际为: %d", res.ExitCode)
	}
}

// 3. 验证超时控制与子进程终止
func TestRunTimeout(t *testing.T) {
	child := getChildExecutable(t)
	ctx := context.Background()

	// 设置超时 100ms，子进程请求睡眠 3000ms
	res, err := Run(ctx, child, []string{"sleep", "3000"}, 100*time.Millisecond, 1024)
	if !errors.Is(err, ErrProcessTimeout) {
		t.Fatalf("预期返回 ErrProcessTimeout，实际为: %v", err)
	}
	if res == nil {
		t.Fatal("超时应返回非空结果")
	}
	if res.ExitCode != -1 {
		t.Fatalf("超时退出码应为 -1，实际为: %d", res.ExitCode)
	}
}

// 4. 验证主动 Context 取消
func TestRunCancel(t *testing.T) {
	child := getChildExecutable(t)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	res, err := Run(ctx, child, []string{"sleep", "3000"}, 5*time.Second, 1024)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("预期返回 ErrCancelled，实际为: %v", err)
	}
	if res == nil || res.ExitCode != -1 {
		t.Fatalf("取消时预期退出码为 -1")
	}
}

// 5. 验证持续洪水输出时的缓冲区上限截断与管道防阻塞
func TestRunOutputLimitAndBackpressure(t *testing.T) {
	child := getChildExecutable(t)
	ctx := context.Background()

	limit := 1024
	// 产生 2000 行，远远超过 1024 字节
	res, err := Run(ctx, child, []string{"flood", "2000"}, 5*time.Second, limit)
	if !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("预期返回 ErrOutputLimit，实际为: %v", err)
	}
	if !res.Truncated {
		t.Fatal("预期 Truncated 为 true")
	}
	if len(res.Stdout) > limit {
		t.Fatalf("输出字节数超出上限: 实际 %d > 上限 %d", len(res.Stdout), limit)
	}
}

// 6. 验证带特殊字符、空格、引号与 & 符号的字面传参（V02 验收核心：防止 Shell 注入）
func TestRunLiteralArguments(t *testing.T) {
	child := getChildExecutable(t)
	ctx := context.Background()

	args := []string{"echo", "hello & calc.exe", "\"quoted\"", "中文参数 带 空格"}
	res, err := Run(ctx, child, args, 5*time.Second, 4096)
	if err != nil {
		t.Fatalf("运行失败: %v", err)
	}

	outStr := string(res.Stdout)
	if !strings.Contains(outStr, "hello & calc.exe") {
		t.Fatalf("未原样保留 & 符号: %s", outStr)
	}
	if !strings.Contains(outStr, "\"quoted\"") {
		t.Fatalf("未原样保留引号: %s", outStr)
	}
	if !strings.Contains(outStr, "中文参数 带 空格") {
		t.Fatalf("未原样保留中文与空格: %s", outStr)
	}
}

// 7. 验证取消操作不会误杀独立运行的同名子进程（V02 验收核心）
func TestRunDoesNotKillUnrelatedProcesses(t *testing.T) {
	child := getChildExecutable(t)

	// 独立启动一个同名夹具进程（睡眠 5 秒）
	independentCmd := exec.Command(child, "sleep", "5000")
	if err := independentCmd.Start(); err != nil {
		t.Fatalf("启动独立进程失败: %v", err)
	}
	independentPid := independentCmd.Process.Pid
	defer func() {
		_ = independentCmd.Process.Kill()
		_ = independentCmd.Wait()
	}()

	// 确保独立进程正在运行
	time.Sleep(50 * time.Millisecond)

	// 使用 runner 启动另一个夹具并立即超时取消
	ctx := context.Background()
	_, _ = Run(ctx, child, []string{"sleep", "3000"}, 100*time.Millisecond, 1024)

	// 检查独立进程是否依然存活，绝不能被误杀
	proc, findErr := os.FindProcess(independentPid)
	if findErr != nil {
		t.Fatalf("查找独立进程失败: %v", findErr)
	}
	if proc == nil {
		t.Fatal("独立进程不应为 nil")
	}

	// 等待一小段时间确认其没有收到意外 kill
	time.Sleep(100 * time.Millisecond)
	// 在 Windows 上，若进程已死，对其操作会报错
	if independentCmd.ProcessState != nil && independentCmd.ProcessState.Exited() {
		t.Fatalf("独立进程 PID %d 被误杀了！", independentPid)
	}
}
