// ===================================================================================
// File: client/main.go
// Description: DDNS 客户端程序 (V2.0)。
// ===================================================================================
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

// AppConfig 用于存储从 config.ini 读取的所有配置项。
type AppConfig struct {
	ServerURL            string
	Username             string
	SecretToken          string
	DomainName           string
	RR                   string
	CheckIntervalSeconds int
}

var config AppConfig

const lastIPFile = "last_ip.txt" // 用于在本地记录上一次的IP地址

// loadConfig 从 config.ini 文件加载运行所需的核心配置。
func loadConfig(isUpdateDaemon bool) error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		return fmt.Errorf("无法加载 config.ini: %v", err)
	}

	clientSection := cfg.Section("client")
	config.ServerURL = clientSection.Key("server_url").String()
	config.Username = clientSection.Key("username").String()
	config.SecretToken = clientSection.Key("secret_token").String()

	// 核心认证信息是所有命令都必需的
	if config.ServerURL == "" || config.Username == "" || config.SecretToken == "" {
		return fmt.Errorf("config.ini 中缺少 server_url, username, 或 secret_token")
	}

	// 只有运行更新守护进程时，才需要域名相关的完整配置
	if isUpdateDaemon {
		config.DomainName = clientSection.Key("domain_name").String()
		config.RR = clientSection.Key("rr").String()
		config.CheckIntervalSeconds = clientSection.Key("check_interval_seconds").MustInt(300)
		if config.DomainName == "" || config.RR == "" {
			return fmt.Errorf("config.ini 中缺少 domain_name 或 rr 配置项")
		}
	}
	return nil
}

// getPublicIP 从多个公共服务获取本机的公网IP，以提高可靠性。
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

// readLastIP 从本地文件 last_ip.txt 读取上次记录的IP地址。
func readLastIP() (string, error) {
	data, err := os.ReadFile(lastIPFile)
	if os.IsNotExist(err) {
		return "", nil // 文件不存在是正常情况，说明是首次运行
	}
	return string(data), err
}

// writeLastIP 将新的IP地址写入本地文件 last_ip.txt。
func writeLastIP(ip string) error {
	return os.WriteFile(lastIPFile, []byte(ip), 0644)
}

// updateRequest 是用于“更新IP”请求的JSON结构体。
type updateRequest struct {
	Username   string `json:"username"`
	DomainName string `json:"domain_name"`
	RR         string `json:"rr"`
	NewIP      string `json:"new_ip"`
}

// manageRequest 是用于“删除域名”请求的JSON结构体。
type manageRequest struct {
	Username   string `json:"username"`
	DomainName string `json:"domain_name,omitempty"`
	RR         string `json:"rr,omitempty"`
}

// main 函数是程序的入口，负责解析命令行标志并分发到不同的处理函数。
func main() {
	log.SetFlags(0) // 移除日志的时间戳，让CLI输出更干净

	// 定义命令行标志
	updateFlag := flag.Bool("update", false, "启动后台守护进程，持续更新IP地址 (默认操作)。")
	listFlag := flag.Bool("list", false, "查询并列出当前用户已注册的所有域名。")
	removeFlag := flag.String("remove", "", "注销一个已注册的域名。用法: -remove <rr.domain.com>")

	// 自定义帮助信息
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "goddns-client: 一个多功能DDNS客户端工具。\n\n")
		fmt.Fprintf(os.Stderr, "使用方法:\n")
		fmt.Fprintf(os.Stderr, "  goddns-client [flags]\n\n")
		fmt.Fprintf(os.Stderr, "默认操作 (不带任何标志) 是启动IP更新守护进程。\n\n")
		fmt.Fprintf(os.Stderr, "可用标志:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// 根据标志决定执行哪个命令
	if *listFlag {
		if err := loadConfig(false); err != nil { // list 命令不需要完整配置
			log.Fatalf("错误: %v", err)
		}
		runList()
	} else if *removeFlag != "" {
		if err := loadConfig(false); err != nil { // remove 命令不需要完整配置
			log.Fatalf("错误: %v", err)
		}
		runRemove(*removeFlag)
	} else {
		// 默认行为是运行更新守护进程
		*updateFlag = true
		if err := loadConfig(true); err != nil { // update 命令需要完整配置
			log.Fatalf("错误: %v", err)
		}
		runUpdateDaemon()
	}
}

// runUpdateDaemon 启动后台守护进程，定期检查并更新IP。
func runUpdateDaemon() {
	log.SetFlags(log.Ldate | log.Ltime) // 为后台日志重新启用时间戳
	log.Println("DDNS 客户端 (V2.0) [更新模式] 启动...")
	log.Printf("配置加载成功: 用户名=%s, 服务端地址=%s, 目标域名=%s.%s, 检查间隔=%v", config.Username, config.ServerURL, config.RR, config.DomainName, time.Duration(config.CheckIntervalSeconds)*time.Second)

	checkAndSendUpdate() // 启动时立即执行一次

	ticker := time.NewTicker(time.Duration(config.CheckIntervalSeconds) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		checkAndSendUpdate()
	}
}

// checkAndSendUpdate 封装了检查并发送IP更新的完整逻辑。
func checkAndSendUpdate() {
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
	if currentIP != lastIP {
		sendUpdateRequest(currentIP)
	} else {
		log.Println("IP 地址未变化，本次无需更新。")
	}
}

// sendUpdateRequest 向服务端发送更新IP的请求。
func sendUpdateRequest(currentIP string) {
	log.Printf("检测到 IP 地址变化，准备为用户 '%s' 发送更新请求...", config.Username)
	reqPayload := updateRequest{
		Username:   config.Username,
		DomainName: config.DomainName,
		RR:         config.RR,
		NewIP:      currentIP,
	}
	jsonData, _ := json.Marshal(reqPayload)
	req, _ := http.NewRequest(http.MethodPost, config.ServerURL+"/update-dns", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.SecretToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("错误: 发送更新请求失败: %v", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		log.Printf("成功: 服务端响应: %s", string(body))
		if err := writeLastIP(currentIP); err != nil {
			log.Printf("严重错误: 更新本地 IP 记录文件 %s 失败: %v", lastIPFile, err)
		}
	} else {
		log.Printf("失败: 服务端返回错误 (状态码: %d): %s", resp.StatusCode, string(body))
	}
}

// runList 执行“查看域名”的功能。
func runList() {
	log.Println("正在向服务端查询已注册的域名列表...")
	req, err := http.NewRequest(http.MethodGet, config.ServerURL+"/manage-records?username="+config.Username, nil)
	if err != nil {
		log.Fatalf("错误: 创建请求失败: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.SecretToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("错误: 发送请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("服务端返回错误 (状态码: %d): %s", resp.StatusCode, string(body))
	}

	var records []struct {
		DomainName string `json:"domain_name"`
		RR         string `json:"rr"`
	}
	if err := json.Unmarshal(body, &records); err != nil {
		log.Fatalf("错误: 解析服务端响应失败: %v", err)
	}

	if len(records) == 0 {
		log.Println("您名下当前没有注册任何域名。")
		return
	}

	log.Println("您已注册的域名如下:")
	for _, r := range records {
		fmt.Printf("- %s.%s\n", r.RR, r.DomainName)
	}
}

// runRemove 执行“删除域名”的功能。
func runRemove(fullDomain string) {
	log.Printf("准备向服务端请求注销域名: %s", fullDomain)
	parts := strings.SplitN(fullDomain, ".", 2)
	if len(parts) < 2 {
		log.Fatalf("域名格式错误。请输入完整域名，例如 'home.example.com'")
	}
	rr, domainName := parts[0], parts[1]

	payload := manageRequest{
		Username:   config.Username,
		DomainName: domainName,
		RR:         rr,
	}
	jsonData, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodDelete, config.ServerURL+"/manage-records", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("错误: 创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.SecretToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("错误: 发送请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("服务端返回错误 (状态码: %d): %s", resp.StatusCode, string(body))
	}

	log.Printf("成功: 服务端响应: %s", string(body))
}
