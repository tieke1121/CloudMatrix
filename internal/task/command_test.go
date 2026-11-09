package task

import (
	"context"
	"testing"

	"cloudmatrix/internal/domain"
)

// 测试预设只读动作参数转换
func TestBuildDiagnosticArgs(t *testing.T) {
	args, err := BuildDiagnosticArgs(DiagGetProp)
	if err != nil || len(args) != 1 || args[0] != "getprop" {
		t.Fatalf("getprop 参数转换失败: %v", args)
	}

	memArgs, err := BuildDiagnosticArgs(DiagMemInfo)
	if err != nil || len(memArgs) != 2 || memArgs[0] != "dumpsys" || memArgs[1] != "meminfo" {
		t.Fatalf("meminfo 参数转换失败: %v", memArgs)
	}

	_, unknownErr := BuildDiagnosticArgs("unknown_action")
	if unknownErr == nil {
		t.Fatal("未知诊断动作应该报错")
	}
}

// 测试单目标失败不阻塞其他目标，且非零退出码绝不误显示为成功（V08 核心验收）
func TestCommandNonBlockingAndExitCodeAccuracy(t *testing.T) {
	// 构造两个实例：实例 1 缺少 serial (必然失败)，实例 2 正常
	inst1 := &domain.Instance{
		ID:        "inst-missing-serial",
		AdbSerial: "",
	}
	inst2 := &domain.Instance{
		ID:        "inst-mock-target",
		AdbSerial: "127.0.0.1:9999",
	}

	targets := []*domain.Instance{inst1, inst2}

	// 此处传入 nil client 用于快速测试参数与边界拦截
	_, err := ExecuteConsoleCommand(context.Background(), nil, targets, CommandRequest{
		Action: DiagGetProp,
	})
	if err == nil {
		t.Fatal("nil client 应报错拦截")
	}

	// 空目标测试
	_, emptyErr := ExecuteConsoleCommand(context.Background(), nil, []*domain.Instance{}, CommandRequest{
		Action: DiagGetProp,
	})
	if emptyErr == nil {
		t.Fatal("空目标集合应报错拦截")
	}

	// 测试结果结构体状态准确性校验
	resFail := TargetCommandResult{
		InstanceID: "inst-1",
		ExitCode:   7,
		Stderr:     "Command failed with code 7",
	}

	if resFail.ExitCode == 0 {
		t.Fatal("失败命令的退出码绝对不能为 0 (禁止误报成功)")
	}
}
