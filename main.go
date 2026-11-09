package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"
	"unsafe"

	"cloudmatrix/internal/config"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// 嵌入前端静态资源
//
//go:embed all:frontend/dist
var assets embed.FS

// showNativeErrorBox 使用 Windows 原生 MessageBox 弹出错误对话框
func showNativeErrorBox(title, message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	procMessageBoxW := user32.NewProc("MessageBoxW")

	titlePtr, _ := syscall.UTF16PtrFromString(title)
	msgPtr, _ := syscall.UTF16PtrFromString(message)

	// MB_ICONERROR (0x00000010) | MB_OK (0x00000000) | MB_SYSTEMMODAL (0x00001000)
	flags := uintptr(0x00000010 | 0x00000000 | 0x00001000)
	_, _, _ = procMessageBoxW.Call(0, uintptr(unsafe.Pointer(msgPtr)), uintptr(unsafe.Pointer(titlePtr)), flags)
}

// writeStartupLog 将启动诊断写入用户配置目录。
func writeStartupLog(msg string) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, msg)
		return
	}
	logDir := filepath.Join(configDir, "CloudMatrix")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, msg, err)
		return
	}
	logFile := filepath.Join(logDir, "startup_error.log")
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("[%s] %s\n", timestamp, msg)
	if err := os.WriteFile(logFile, []byte(entry), 0644); err != nil {
		fmt.Fprintln(os.Stderr, msg, err)
	}
}

func main() {
	// 全局崩溃保护与日志捕获
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			errDetails := fmt.Sprintf("CloudMatrix 启动过程中发生未捕获异常:\n\n%v\n\n调用堆栈:\n%s", r, stack)
			writeStartupLog(errDetails)
			showNativeErrorBox("CloudMatrix 启动崩溃", errDetails)
		}
	}()

	cfg, err := config.Load(os.Getenv("CLOUDMATRIX_CONFIG"))
	if err != nil {
		writeStartupLog(err.Error())
		showNativeErrorBox("CloudMatrix 配置错误", err.Error())
		os.Exit(1)
	}
	app, err := NewApp(cfg)
	if err != nil {
		writeStartupLog(err.Error())
		showNativeErrorBox("CloudMatrix 初始化失败", err.Error())
		os.Exit(1)
	}

	// 启动 Wails 窗口应用
	err = wails.Run(&options.App{
		Title:  "CloudMatrix 实例管理平台",
		Width:  1024,
		Height: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 247, B: 250, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})
	app.shutdown()

	if err != nil {
		errDetails := fmt.Sprintf("Wails 桌面引擎启动失败:\n\n%v", err)
		writeStartupLog(errDetails)
		showNativeErrorBox("CloudMatrix 启动错误", errDetails)
	}
}
