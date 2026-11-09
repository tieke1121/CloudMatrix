package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cloudmatrix/internal/adb"
	"cloudmatrix/internal/domain"
	"cloudmatrix/internal/observe"
)

// DiagnosticAction 只读诊断动作类型
type DiagnosticAction string

const (
	DiagGetProp DiagnosticAction = "getprop"
	DiagMemInfo DiagnosticAction = "meminfo"
	DiagNetstat DiagnosticAction = "netstat"
	DiagTop     DiagnosticAction = "top"
)

// CommandRequest 控制台执行请求
type CommandRequest struct {
	Targets     []string         `json:"targets"`
	Action      DiagnosticAction `json:"action,omitempty"`
	RawArgs     []string         `json:"raw_args,omitempty"`
	Timeout     time.Duration    `json:"timeout"`
	OutputLimit int              `json:"output_limit"`
}

// TargetCommandResult 单目标执行结果
type TargetCommandResult struct {
	InstanceID string `json:"instance_id"`
	Serial     string `json:"serial"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Truncated  bool   `json:"truncated"`
	Error      string `json:"error,omitempty"`
}

// BuildDiagnosticArgs 将预设诊断动作转换为安全的 Android Shell 参数
func BuildDiagnosticArgs(action DiagnosticAction) ([]string, error) {
	switch action {
	case DiagGetProp:
		return []string{"getprop"}, nil
	case DiagMemInfo:
		return []string{"dumpsys", "meminfo"}, nil
	case DiagNetstat:
		return []string{"netstat", "-n"}, nil
	case DiagTop:
		return []string{"top", "-n", "1"}, nil
	default:
		return nil, fmt.Errorf("未知的只读诊断动作: %s", action)
	}
}

// ExecuteConsoleCommand 批量执行控制台命令，严格限制为 Android Shell 语法，杜绝宿主注入
func ExecuteConsoleCommand(
	ctx context.Context,
	client *adb.Client,
	targetInstances []*domain.Instance,
	req CommandRequest,
) ([]*TargetCommandResult, error) {
	if client == nil {
		return nil, errors.New("ADB 客户端未就绪")
	}
	if len(targetInstances) == 0 {
		return nil, domain.NewPlatformError(domain.ErrCodeInvalidArgument, "执行目标集合不能为空", false)
	}

	// 确定待执行的 Android 参数切片
	var shellArgs []string
	if req.Action != "" {
		args, err := BuildDiagnosticArgs(req.Action)
		if err != nil {
			return nil, err
		}
		shellArgs = args
	} else if len(req.RawArgs) > 0 {
		shellArgs = req.RawArgs
	} else {
		return nil, domain.NewPlatformError(domain.ErrCodeInvalidArgument, "必须指定诊断动作或命令参数", false)
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 300*time.Second {
		timeout = 300 * time.Second
	}

	outputLimit := req.OutputLimit
	if outputLimit <= 0 {
		outputLimit = 512 * 1024
	}

	results := make([]*TargetCommandResult, len(targetInstances))
	var wg sync.WaitGroup

	for i, inst := range targetInstances {
		i := i
		inst := inst
		wg.Add(1)

		go func() {
			defer wg.Done()
			resItem := &TargetCommandResult{
				InstanceID: inst.ID,
				Serial:     inst.AdbSerial,
			}

			if inst.AdbSerial == "" {
				resItem.ExitCode = 1
				resItem.Error = "实例未绑定有效 ADB serial"
				results[i] = resItem
				return
			}

			// 直接将参数传递给 adb shell，不经过宿主 cmd/powershell
			execRes, err := client.RunShell(ctx, inst.AdbSerial, shellArgs, timeout)
			if err != nil {
				resItem.ExitCode = 1
				resItem.Error = observe.RedactString(err.Error())
				if execRes != nil {
					resItem.ExitCode = execRes.ExitCode
					resItem.Stdout = observe.RedactString(string(execRes.Stdout))
					resItem.Stderr = observe.RedactString(string(execRes.Stderr))
					resItem.Truncated = execRes.Truncated
				}
			} else {
				resItem.ExitCode = execRes.ExitCode
				resItem.Stdout = observe.RedactString(string(execRes.Stdout))
				resItem.Stderr = observe.RedactString(string(execRes.Stderr))
				resItem.Truncated = execRes.Truncated
			}

			results[i] = resItem
		}()
	}

	wg.Wait()
	return results, nil
}
