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
)

func main() {
	log.Println("DDNS 服务端 (V2.1 - 端口可配置) 启动中...")

	// 启动时先加载服务自身配置 (如端口号)
	if err := config.LoadServerConfig(); err != nil {
		log.Fatalf("错误: 启动时加载服务端配置失败: %v", err)
	}

	// 然后加载用户配置
	if err := config.LoadUsers(); err != nil {
		log.Fatalf("错误: 启动时加载用户配置失败: %v", err)
	}

	// 创建一个新的 ServeMux 来精细控制路由
	mux := http.NewServeMux()
	mux.HandleFunc("/update-dns", handler.HandleUpdateDNS)
	mux.HandleFunc("/manage-records", handler.HandleManageRecords)

	server := &http.Server{
		Addr:         ":" + config.ServerPort, // 使用从配置文件读取的端口
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	log.Printf("将在端口 %s 上监听请求", config.ServerPort)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("错误: 启动 HTTP 服务器失败: %v", err)
	}
}
