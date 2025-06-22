// ===================================================================================
// File: client/main.go
// Description: DDNS 客户端程序 (V2.0 )。
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

// AppConfig holds the configuration read from config.ini
type AppConfig struct {
	ServerURL            string
	Username             string
	SecretToken          string
	CheckIntervalSeconds int
}

var config AppConfig

const lastIPFile = "last_ip.txt"

// loadConfig loads configuration from config.ini
func loadConfig() error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		return fmt.Errorf("无法加载 config.ini: %v", err)
	}

	clientSection := cfg.Section("client")
	config.ServerURL = clientSection.Key("server_url").String()
	config.Username = clientSection.Key("username").String()
	config.SecretToken = clientSection.Key("secret_token").String()
	config.CheckIntervalSeconds = clientSection.Key("check_interval_seconds").MustInt(300)

	if config.ServerURL == "" || config.Username == "" || config.SecretToken == "" {
		return fmt.Errorf("config.ini 中缺少 server_url, username, 或 secret_token")
	}
	return nil
}

// getPublicIP gets the public IP from an external service
func getPublicIP() (string, error) {
	ipServices := []string{
		"[https://api.ipify.org](https://api.ipify.org)",
		"[http://ifconfig.me/ip](http://ifconfig.me/ip)",
		"[http://icanhazip.com](http://icanhazip.com)",
		"[http://ipinfo.io/ip](http://ipinfo.io/ip)",
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

// readLastIP reads the last known IP from a local file
func readLastIP() (string, error) {
	data, err := os.ReadFile(lastIPFile)
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

// writeLastIP writes the new IP to the local file
func writeLastIP(ip string) error {
	return os.WriteFile(lastIPFile, []byte(ip), 0644)
}

// UpdateRequest is the structure for the request sent to the server
type updateRequest struct {
	Username string `json:"username"`
	NewIP    string `json:"new_ip"`
}

// sendUpdateRequest sends an update request to the server
func sendUpdateRequest(currentIP string) {
	log.Printf("检测到 IP 地址变化，准备为用户 '%s' 发送更新请求...", config.Username)

	reqPayload := updateRequest{
		Username: config.Username,
		NewIP:    currentIP,
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

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("错误: 发送更新请求到 %s 失败: %v", config.ServerURL, err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK {
		log.Printf("成功: 服务端响应: %s", string(respBody))
		if err := writeLastIP(currentIP); err != nil {
			log.Printf("严重错误: 更新本地 IP 记录文件 %s 失败: %v", lastIPFile, err)
		}
	} else {
		log.Printf("失败: 服务端返回错误 (状态码: %d): %s", resp.StatusCode, string(respBody))
	}
}

// checkAndupdate is the main logic to check and update the IP
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
	log.Println("DDNS 客户端 (V2.0) 启动...")
	if err := loadConfig(); err != nil {
		log.Fatalf("错误: 加载配置失败: %v", err)
	}
	log.Printf("配置加载成功: 用户名=%s, 服务端地址=%s, 检查间隔=%v", config.Username, config.ServerURL, time.Duration(config.CheckIntervalSeconds)*time.Second)

	checkAndupdate()

	ticker := time.NewTicker(time.Duration(config.CheckIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		checkAndupdate()
	}
}
