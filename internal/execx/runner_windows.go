//go:build windows

package execx

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	// currentJobMutex 保护 JobObject 关联状态
	jobStateMap sync.Map
)

// setupPlatformProcess 在 Windows 上创建带有关闭即退出全部子进程特性的 JobObject
func setupPlatformProcess(cmd *exec.Cmd) (func(), error) {
	// 创建匿名作业对象
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return func() {}, err
	}

	// 配置作业属性：当作业句柄关闭时，终止该作业树内的所有子进程
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}

	_, setErr := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	if setErr != nil {
		windows.CloseHandle(job)
		return func() {}, setErr
	}

	// 记录 cmd 与 Job 句柄的关系
	jobStateMap.Store(cmd, job)

	cleaner := func() {
		if val, ok := jobStateMap.LoadAndDelete(cmd); ok {
			hJob := val.(windows.Handle)
			// 主动终结作业中的全部进程，再关闭句柄
			windows.TerminateJobObject(hJob, 1)
			windows.CloseHandle(hJob)
		}
	}

	return cleaner, nil
}

// attachPlatformProcess 将已启动的子进程绑定到该命令对应的 JobObject 中
func attachPlatformProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	val, ok := jobStateMap.Load(cmd)
	if !ok {
		return nil
	}

	job := val.(windows.Handle)
	// 分配配额与终止权限
	access := uint32(windows.PROCESS_SET_QUOTA | windows.PROCESS_TERMINATE)
	procHandle, err := windows.OpenProcess(access, false, uint32(cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(procHandle)

	return windows.AssignProcessToJobObject(job, procHandle)
}
