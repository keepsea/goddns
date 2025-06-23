// ===================================================================================
// File: ddns-client/cmd/update.go
// Description: 负责执行IP更新的核心逻辑。
// ===================================================================================
package cmd

import (
	"log"
	"net/http"
	"time"

	"github.com/keepsea/goddns/ddns_client/api"
	"github.com/keepsea/goddns/ddns_client/config"
	"github.com/keepsea/goddns/ddns_client/util"
)

type updateRequest struct {
	SecretToken string `json:"secret_token"`
	DomainName  string `json:"domain_name"`
	RR          string `json:"rr"`
	NewIP       string `json:"new_ip"`
}

func RunUpdateDaemon() {
	log.SetFlags(log.Ldate | log.Ltime)
	log.Println("DDNS 客户端 (V2.2) [更新模式] 启动...")
	log.Printf("配置加载成功: 用户名=%s, 服务端地址=%s, 目标域名=%s.%s, 检查间隔=%v", config.App.Username, config.App.ServerURL, config.App.RR, config.App.DomainName, time.Duration(config.App.CheckIntervalSeconds)*time.Second)

	checkAndSendUpdate()

	ticker := time.NewTicker(time.Duration(config.App.CheckIntervalSeconds) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		checkAndSendUpdate()
	}
}

func checkAndSendUpdate() {
	log.Println("开始检查公网 IP...")
	currentIP, err := util.GetPublicIP()
	if err != nil {
		log.Printf("错误: 获取公网 IP 失败: %v", err)
		return
	}
	log.Printf("当前公网 IP: %s", currentIP)

	lastIP, err := util.ReadLastIP()
	if err != nil {
		log.Printf("错误: 读取本地 IP 记录失败: %v", err)
	}

	if currentIP != lastIP {
		log.Printf("检测到 IP 地址变化，为用户 '%s' 发送更新请求...", config.App.Username)
		payload := updateRequest{
			SecretToken: config.App.SecretToken,
			DomainName:  config.App.DomainName,
			RR:          config.App.RR,
			NewIP:       currentIP,
		}
		body, err := api.SendSecureRequest("/update-dns", http.MethodPost, payload)
		if err != nil {
			log.Printf("失败: %v", err)
			return
		}
		log.Printf("成功: 服务端响应: %s", string(body))
		if err := util.WriteLastIP(currentIP); err != nil {
			log.Printf("严重错误: 更新本地 IP 记录文件失败: %v", err)
		}
	} else {
		log.Println("IP 地址未变化，本次无需更新。")
	}
}
