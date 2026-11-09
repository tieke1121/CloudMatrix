package execx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// 标准分类错误定义
var (
	// ErrToolMissing 可执行工具未找到或路径不存在
	ErrToolMissing = errors.New("TOOL_MISSING: 指定的可执行工具不存在")
	// ErrProcessTimeout 进程执行超出指定时间
	ErrProcessTimeout = errors.New("PROCESS_TIMEOUT: 进程执行超出指定时间")
	// ErrCancelled 进程执行被取消
	ErrCancelled = errors.New("CANCELLED: 进程执行被取消")
	// ErrOutputLimit 进程输出超出预设安全上限
	ErrOutputLimit = errors.New("OUTPUT_LIMIT: 进程输出超出安全上限")
)

// Result 进程执行结果
type Result struct {
	// ExitCode 进程退出码
	ExitCode int
	// Stdout 标准输出字节流（受限截断）
	Stdout []byte
	// Stderr 标准错误字节流（受限截断）
	Stderr []byte
	// Truncated 是否发生输出截断
	Truncated bool
	// Duration 实际耗时
	Duration time.Duration
}

// limitBuffer 具备最大字节数限制且能持续排空管道的缓冲区
type limitBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

// Write 实现 io.Writer，超过限制后持续排空以防阻塞子进程管道
func (l *limitBuffer) Write(p []byte) (n int, err error) {
	n = len(p)
	remaining := l.limit - l.buf.Len()
	if remaining > 0 {
		if len(p) <= remaining {
			l.buf.Write(p)
		} else {
			l.buf.Write(p[:remaining])
			l.truncated = true
		}
	} else {
		l.truncated = true
	}
	return n, nil
}

// Bytes 返回已记录的字节切片
func (l *limitBuffer) Bytes() []byte {
	return l.buf.Bytes()
}

// Run 执行指定工具与参数，严格杜绝 Shell 拼接，支持超时、取消与输出背压保护
func Run(ctx context.Context, executable string, args []string, timeout time.Duration, outputLimit int) (*Result, error) {
	if ctx == nil {
		return nil, errors.New("process context is required")
	}
	if ctx.Err() != nil {
		return nil, ErrCancelled
	}
	if executable == "" {
		return nil, ErrToolMissing
	}
	// 校验可执行文件路径是否存在
	if _, err := os.Stat(executable); err != nil {
		// 若不是绝对路径，尝试从 PATH 查找
		if _, lookErr := exec.LookPath(executable); lookErr != nil {
			return nil, fmt.Errorf("%w: %s", ErrToolMissing, executable)
		}
	}

	if outputLimit <= 0 {
		outputLimit = 1024 * 1024
	}

	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.Command(executable, args...)
	cmd.WaitDelay = 2 * time.Second

	stdoutBuf := &limitBuffer{limit: outputLimit}
	stderrBuf := &limitBuffer{limit: outputLimit}
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	// 平台特定的进程树归属与隔离初始化（Windows JobObject 绑定）
	jobCleaner, err := setupPlatformProcess(cmd)
	if err != nil {
		return nil, fmt.Errorf("初始化平台进程树失败: %w", err)
	}
	defer jobCleaner()

	startTime := time.Now()
	startErr := cmd.Start()
	if startErr != nil {
		return nil, fmt.Errorf("启动进程失败: %w", startErr)
	}

	// 附加进程到当前平台的作业树
	attachErr := attachPlatformProcess(cmd)

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// 超时或上下文主动取消，调用平台清理器终结整个进程树
		jobCleaner()
		if attachErr != nil {
			// A short-lived process can exit before it is assigned to the job.
			// If it is still running, terminate that exact process.
			if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return nil, fmt.Errorf("取消进程失败: %w", err)
			}
		}
		<-done
		duration := time.Since(startTime)
		res := &Result{
			ExitCode:  -1,
			Stdout:    stdoutBuf.Bytes(),
			Stderr:    stderrBuf.Bytes(),
			Truncated: stdoutBuf.truncated || stderrBuf.truncated,
			Duration:  duration,
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return res, ErrProcessTimeout
		}
		return res, ErrCancelled

	case waitErr := <-done:
		duration := time.Since(startTime)
		exitCode := 0
		if waitErr != nil {
			var exitErr *exec.ExitError
			if errors.As(waitErr, &exitErr) {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}

		isTruncated := stdoutBuf.truncated || stderrBuf.truncated
		res := &Result{
			ExitCode:  exitCode,
			Stdout:    stdoutBuf.Bytes(),
			Stderr:    stderrBuf.Bytes(),
			Truncated: isTruncated,
			Duration:  duration,
		}

		if isTruncated {
			return res, ErrOutputLimit
		}

		return res, nil
	}
}
