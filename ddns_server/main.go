// ===================================================================================
// File: ddns-server/main.go
// Description: 项目主入口，负责初始化和启动服务。
// ===================================================================================
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/keepsea/goddns/ddns_server/config"
	"github.com/keepsea/goddns/ddns_server/handler"
	"github.com/keepsea/goddns/ddns_server/security"
)

func main() {
	log.Println("GODDNS 服务端 (V2.1.0) 启动中...")

	// 启动时加载所有配置
	if err := config.LoadServerConfig(); err != nil {
		log.Fatalf("错误: 启动时加载服务端配置失败: %v", err)
	}
	if err := config.LoadUsers(); err != nil {
		log.Fatalf("错误: 启动时加载用户配置失败: %v", err)
	}

	// 创建一个新的 ServeMux 来精细控制路由
	mux := http.NewServeMux()
	mux.HandleFunc("/update-dns", handler.HandleUpdateDNS)
	mux.HandleFunc("/manage-records", handler.HandleManageRecords)
	mux.HandleFunc("/manage-key", handler.HandleManageKey)

	// 应用我们的安全中间件
	// 1. 限制请求体大小为1MB
	// 2. 对每个IP进行速率限制
	handlerWithMiddleware := security.LimitRequestSize(mux, 1024*1024)
	handlerWithMiddleware = security.RateLimit(handlerWithMiddleware)

	server := &http.Server{
		Addr:         ":" + config.ServerPort,
		Handler:      handlerWithMiddleware,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	log.Printf("将在端口 %s 上监听请求", config.ServerPort)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("错误: 启动 HTTP 服务器失败: %v", err)
	}
}
