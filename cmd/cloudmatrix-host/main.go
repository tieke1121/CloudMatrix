package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"cloudmatrix/api"
	"cloudmatrix/internal/transport"
)

func main() {
	listenAddr := flag.String("listen", "127.0.0.1:18890", "Host Agent 监听地址")
	hostID := flag.String("host-id", "", "宿主机唯一标识 (默认自动获取主机名)")
	flag.Parse()
	host, _, err := net.SplitHostPort(*listenAddr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		log.Fatal("[HostAgent] 当前版本只允许监听显式回环地址（127.0.0.1 或 [::1]）")
	}

	if *hostID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "cloudmatrix-host-unknown"
		}
		*hostID = hostname
	}

	log.Printf("[HostAgent] 启动 CloudMatrix 远程宿主节点服务 (Version: %s)...", api.RPCVersion)
	log.Printf("[HostAgent] Host ID: %s, 监听地址: %s", *hostID, *listenAddr)

	log.Print("[HostAgent] 任务执行器未接入；仅提供本机探活")
	server := transport.NewRPCServer(*hostID, os.Getenv("CLOUDMATRIX_HOST_TOKEN"), nil)
	if err := server.Start(*listenAddr); err != nil {
		log.Fatalf("[HostAgent] 启动监听失败: %v", err)
	}

	// 监听中断退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Printf("[HostAgent] 收到停止信号，正在关闭服务...")
	server.Stop()
	log.Printf("[HostAgent] 服务已完全退出")
}
