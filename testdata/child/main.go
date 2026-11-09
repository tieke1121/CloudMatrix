package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// 子进程测试夹具，覆盖成功、异常退出码、挂起、洪水输出、派生孙进程与特殊参数
func main() {
	if len(os.Args) < 2 {
		fmt.Println("缺少模式参数")
		os.Exit(1)
	}

	mode := os.Args[1]

	switch mode {
	case "exit":
		// 退出码模式
		code := 0
		if len(os.Args) >= 3 {
			if c, err := strconv.Atoi(os.Args[2]); err == nil {
				code = c
			}
		}
		os.Exit(code)

	case "sleep":
		// 挂起/睡眠模式
		ms := 5000
		if len(os.Args) >= 3 {
			if m, err := strconv.Atoi(os.Args[2]); err == nil {
				ms = m
			}
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
		os.Exit(0)

	case "flood":
		// 持续输出模式，测试输出缓冲区背压与截断
		count := 1000
		if len(os.Args) >= 3 {
			if c, err := strconv.Atoi(os.Args[2]); err == nil {
				count = c
			}
		}
		line := "CloudMatrixProcessOutputTestLine1234567890abcdefghijklmnopqrstuvwxyz\n"
		for i := 0; i < count; i++ {
			os.Stdout.WriteString(line)
		}
		os.Exit(0)

	case "echo":
		// 参数原样回显模式
		for i := 2; i < len(os.Args); i++ {
			fmt.Println(os.Args[i])
		}
		os.Exit(0)

	case "spawn":
		// 派生孙进程模式，测试 JobObject 树级进程回收
		sleepMs := "10000"
		if len(os.Args) >= 3 {
			sleepMs = os.Args[2]
		}
		cmd := exec.Command(os.Args[0], "sleep", sleepMs)
		if err := cmd.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "派生孙进程失败: %v\n", err)
			os.Exit(1)
		}
		// 父进程立即退出，留下孙进程在后台运行
		os.Exit(0)

	default:
		fmt.Printf("未知模式: %s\n", mode)
		os.Exit(2)
	}
}
