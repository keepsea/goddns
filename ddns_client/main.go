// ===================================================================================
// File: client/main.go
// Description: DDNS 客户端程序，部署在家庭宽带服务器上。
// ===================================================================================
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

// AppConfig 保存从 config.ini 读取的配置
type AppConfig struct {
	ServerURL            string
	SecretToken          string
	DomainName           string
	RR                   string
	CheckIntervalSeconds int
}

var config AppConfig

const lastIPFile = "last_ip.txt"

// loadConfig 从 config.ini 文件加载配置
func loadConfig() error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		return fmt.Errorf("无法加载 config.ini: %v", err)
	}

	clientSection := cfg.Section("client")
	config.ServerURL = clientSection.Key("server_url").String()
	config.SecretToken = clientSection.Key("secret_token").String()
	config.DomainName = clientSection.Key("domain_name").String()
	config.RR = clientSection.Key("rr").String()
	config.CheckIntervalSeconds = clientSection.Key("check_interval_seconds").MustInt(300)

	if config.ServerURL == "" || config.SecretToken == "" || config.DomainName == "" || config.RR == "" {
		return fmt.Errorf("config.ini 中缺少一个或多个必要的配置项")
	}
	return nil
}

// getPublicIP 从公共服务获取本机的公网 IP
func getPublicIP() (string, error) {
	ipServices := []string{
		"https://api.ipify.org",
		"http://ifconfig.me/ip",
		"http://icanhazip.com",
		"http://ipinfo.io/ip",
	}

	for _, service := range ipServices {
		resp, err := http.Get(service)
		if err != nil {
			log.Printf("警告: 访问 %s 失败: %v", service, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("警告: %s 返回状态码 %d", service, resp.StatusCode)
			continue
		}

		ip, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("警告: 读取 %s 响应失败: %v", service, err)
			continue
		}
		return strings.TrimSpace(string(ip)), nil
	}

	return "", fmt.Errorf("尝试了所有 IP 服务，均未能获取公网 IP")
}

// readLastIP 从本地文件读取上次记录的 IP
func readLastIP() (string, error) {
	data, err := os.ReadFile(lastIPFile)
	if os.IsNotExist(err) {
		return "", nil // 文件不存在是正常情况
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeLastIP 将新的 IP 写入本地文件
func writeLastIP(ip string) error {
	return os.WriteFile(lastIPFile, []byte(ip), 0644)
}

// UpdateRequest 是发送给服务端的请求结构体
type updateRequest struct {
	DomainName string `json:"domain_name"`
	RR         string `json:"rr"`
	NewIP      string `json:"new_ip"`
}

// sendUpdateRequest 向服务端发送更新请求
func sendUpdateRequest(currentIP string) {
	log.Printf("检测到 IP 地址变化，准备向服务端发送更新请求...")

	reqPayload := updateRequest{
		DomainName: config.DomainName,
		RR:         config.RR,
		NewIP:      currentIP,
	}

	jsonData, err := json.Marshal(reqPayload)
	if err != nil {
		log.Printf("错误: 序列化请求体失败: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, config.ServerURL, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("错误: 创建 HTTP 请求失败: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.SecretToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("错误: 发送更新请求到 %s 失败: %v", config.ServerURL, err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		log.Printf("成功: 服务端响应: %s", string(respBody))
		// 更新成功后，才将新 IP 写入本地文件
		if err := writeLastIP(currentIP); err != nil {
			log.Printf("严重错误: 更新本地 IP 记录文件 %s 失败: %v", lastIPFile, err)
		}
	} else {
		log.Printf("失败: 服务端返回错误 (状态码: %d): %s", resp.StatusCode, string(respBody))
	}
}

// checkAndupdate 检查并更新 IP 的主逻辑
func checkAndupdate() {
	log.Println("开始检查公网 IP...")

	currentIP, err := getPublicIP()
	if err != nil {
		log.Printf("错误: 获取公网 IP 失败: %v", err)
		return
	}
	log.Printf("当前公网 IP: %s", currentIP)

	lastIP, err := readLastIP()
	if err != nil {
		log.Printf("错误: 读取本地 IP 记录失败: %v", err)
	}
	log.Printf("上次记录的 IP: %s", lastIP)

	if currentIP != lastIP {
		sendUpdateRequest(currentIP)
	} else {
		log.Println("IP 地址未变化，本次无需更新。")
	}
}

func main() {
	log.Println("DDNS 客户端启动...")
	if err := loadConfig(); err != nil {
		log.Fatalf("错误: 加载配置失败: %v", err)
	}
	log.Printf("配置信息: 服务端地址=%s, 域名=%s.%s, 检查间隔=%v", config.ServerURL, config.RR, config.DomainName, time.Duration(config.CheckIntervalSeconds)*time.Second)

	checkAndupdate()

	ticker := time.NewTicker(time.Duration(config.CheckIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		checkAndupdate()
	}
}
