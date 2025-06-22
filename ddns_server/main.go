// ===================================================================================
// File: server/main.go
// Description: DDNS 服务端程序。
// ===================================================================================
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	alidns20150109 "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/credentials-go/credentials"
	"gopkg.in/ini.v1"
)

// AppConfig 保存从 config.ini 读取的配置
type AppConfig struct {
	ListenPort  string
	SecretToken string
}

var config AppConfig

// UpdateRequest 是客户端发来的请求结构体
type UpdateRequest struct {
	DomainName string `json:"domain_name"`
	RR         string `json:"rr"`
	NewIP      string `json:"new_ip"`
}

// loadConfig 从 config.ini 文件加载配置
func loadConfig() error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		return fmt.Errorf("无法加载 config.ini: %v", err)
	}

	serverSection := cfg.Section("server")
	config.ListenPort = serverSection.Key("listen_port").MustString("9876")
	config.SecretToken = serverSection.Key("secret_token").String()

	if config.SecretToken == "" {
		return fmt.Errorf("config.ini 中缺少必要的配置: secret_token")
	}

	return nil
}

// createAliyunClient 使用官方推荐的凭据方式创建并返回一个阿里云 DNS 客户端实例
func createAliyunClient() (*alidns20150109.Client, error) {
	// 使用官方推荐的 NewCredential 方法。
	// 它会自动遵循一个凭据链来寻找认证信息：
	// 1. 环境变量 (ALIBABA_CLOUD_ACCESS_KEY_ID / ALIBABA_CLOUD_ACCESS_KEY_SECRET)
	// 2. 配置文件 (~/.alibabacloud/credentials)
	// 3. ECS 实例上的 RAM 角色
	// 这使得我们的应用更加灵活和安全。
	cred, err := credentials.NewCredential(nil)
	if err != nil {
		return nil, err
	}

	// 将获取到的凭据对象传入配置
	aliConfig := &openapi.Config{
		Credential: cred,
	}
	// Endpoint 请参考 https://api.aliyun.com/product/Alidns
	aliConfig.Endpoint = tea.String("dns.aliyuncs.com")
	client, err := alidns20150109.NewClient(aliConfig)
	return client, err
}

// findDomainRecordID 查找指定子域名的 RecordId
func findDomainRecordID(client *alidns20150109.Client, domainName, rr string) (*string, *string, error) {
	req := &alidns20150109.DescribeDomainRecordsRequest{
		DomainName: tea.String(domainName),
		RRKeyWord:  tea.String(rr),
		Type:       tea.String("A"),
	}
	resp, err := client.DescribeDomainRecords(req)
	if err != nil {
		return nil, nil, err
	}

	if *resp.Body.TotalCount == 0 {
		return nil, nil, fmt.Errorf("未找到子域名 '%s' 的 A 记录", rr)
	}

	record := resp.Body.DomainRecords.Record[0]
	return record.RecordId, record.Value, nil
}

// updateDomainRecord 更新 DNS 记录
func updateDomainRecord(client *alidns20150109.Client, recordId, rr, newIP string) error {
	req := &alidns20150109.UpdateDomainRecordRequest{
		RecordId: tea.String(recordId),
		RR:       tea.String(rr),
		Type:     tea.String("A"),
		Value:    tea.String(newIP),
	}
	_, err := client.UpdateDomainRecord(req)
	return err
}

// handleUpdateDNS 是处理 DNS 更新请求的 HTTP Handler
func handleUpdateDNS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	token := r.Header.Get("Authorization")
	if token != "Bearer "+config.SecretToken {
		http.Error(w, "认证失败", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "无法读取请求体", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var req UpdateRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "请求体 JSON 格式错误", http.StatusBadRequest)
		return
	}

	log.Printf("收到更新请求: 域名=%s.%s, IP=%s", req.RR, req.DomainName, req.NewIP)

	client, err := createAliyunClient()
	if err != nil {
		log.Printf("错误: 创建阿里云客户端失败: %v", err)
		http.Error(w, "服务端配置错误", http.StatusInternalServerError)
		return
	}

	recordID, currentIP, err := findDomainRecordID(client, req.DomainName, req.RR)
	if err != nil {
		log.Printf("错误: 查找域名记录失败: %v", err)
		http.Error(w, fmt.Sprintf("查找域名记录失败: %v", err), http.StatusInternalServerError)
		return
	}

	if *currentIP == req.NewIP {
		log.Printf("IP 地址未变化 (%s)，无需更新。", req.NewIP)
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status": "success", "message": "IP 地址未变化，无需更新"}`)
		return
	}

	err = updateDomainRecord(client, *recordID, req.RR, req.NewIP)
	if err != nil {
		log.Printf("错误: 更新域名记录失败: %v", err)
		http.Error(w, fmt.Sprintf("更新域名记录失败: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("成功: 域名 %s.%s 的 IP 已更新为 %s", req.RR, req.DomainName, req.NewIP)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "success", "message": "DNS 记录更新成功"}`)
}

func main() {
	log.Println("DDNS 服务端启动中...")
	if err := loadConfig(); err != nil {
		log.Fatalf("错误: 加载配置失败: %v", err)
	}
	log.Printf("配置加载成功: 端口=%s", config.ListenPort)

	http.HandleFunc("/update-dns", handleUpdateDNS)
	err := http.ListenAndServe(":"+config.ListenPort, nil)
	if err != nil {
		log.Fatalf("错误: 启动 HTTP 服务器失败: %v", err)
	}
}
