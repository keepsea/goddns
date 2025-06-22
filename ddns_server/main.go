// ===================================================================================
// ！！！重要！！！
// 这是 "main.go" 文件的内容。请不要将其他文件的内容粘贴到这个文件的末尾。
// File: server/main.go
// Description: DDNS 服务端程序 (V2.0)，支持多用户管理。
// ===================================================================================
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	//alidns20150109 "github.com/alibabacloud-go/alidns-20150109/v4/client"
	//openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	//"github.com/alibabacloud-go/tea"
	//"github.com/aliyun/credentials-go/credentials"
	alidns20150109 "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/credentials-go/credentials"
)

// --- Server Configuration ---
const (
	listenPort      = "9876"
	usersConfigFile = "users.json"
)

// User defines the structure for a single user's configuration
type User struct {
	Username    string `json:"username"`
	SecretToken string `json:"secret_token"`
	DomainName  string `json:"domain_name"`
	RR          string `json:"rr"` // Host record, e.g., "home" or "@"
}

// UserConfig holds the list of users
type UserConfig struct {
	Users []User `json:"users"`
}

// Global user map for quick lookups
var (
	userMap      map[string]User
	userMapMutex = &sync.RWMutex{}
)

// UpdateRequest is the structure of the request from the client
type UpdateRequest struct {
	Username string `json:"username"`
	NewIP    string `json:"new_ip"`
}

// loadUsers loads user configurations from users.json into the userMap
func loadUsers() error {
	file, err := os.ReadFile(usersConfigFile)
	if err != nil {
		return fmt.Errorf("无法读取用户配置文件 %s: %w", usersConfigFile, err)
	}

	var userConfig UserConfig
	if err := json.Unmarshal(file, &userConfig); err != nil {
		return fmt.Errorf("解析用户配置文件JSON失败: %w", err)
	}

	// Use a temporary map to build the new user list
	tempUserMap := make(map[string]User)
	for _, user := range userConfig.Users {
		if user.Username == "" || user.SecretToken == "" || user.DomainName == "" || user.RR == "" {
			log.Printf("警告: 用户 '%s' 的配置不完整，已跳过。", user.Username)
			continue
		}
		tempUserMap[user.Username] = user
	}

	// Lock the mutex and swap the maps
	userMapMutex.Lock()
	userMap = tempUserMap
	userMapMutex.Unlock()

	log.Printf("成功加载 %d 个用户配置。", len(userMap))
	return nil
}

// createAliyunClient creates and returns an Aliyun DNS client instance
func createAliyunClient() (*alidns20150109.Client, error) {
	cred, err := credentials.NewCredential(nil)
	if err != nil {
		return nil, err
	}

	aliConfig := &openapi.Config{
		Credential: cred,
		Endpoint:   tea.String("dns.aliyuncs.com"),
	}
	client, err := alidns20150109.NewClient(aliConfig)
	return client, err
}

// findDomainRecordID finds the RecordId for a given subdomain
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

	// Exact match for the RR
	for _, record := range resp.Body.DomainRecords.Record {
		if *record.RR == rr {
			return record.RecordId, record.Value, nil
		}
	}

	return nil, nil, fmt.Errorf("未找到完全匹配的主机记录 '%s'", rr)
}

// updateDomainRecord updates a DNS record
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

// handleUpdateDNS handles the DNS update HTTP request
func handleUpdateDNS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	// 1. Parse request body
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

	if req.Username == "" || req.NewIP == "" {
		http.Error(w, "请求体中缺少 'username' 或 'new_ip'", http.StatusBadRequest)
		return
	}

	// 2. Authenticate user
	userMapMutex.RLock()
	user, ok := userMap[req.Username]
	userMapMutex.RUnlock()

	if !ok {
		log.Printf("认证失败: 用户 '%s' 不存在。", req.Username)
		http.Error(w, "认证失败: 无效的用户", http.StatusUnauthorized)
		return
	}

	token := r.Header.Get("Authorization")
	expectedToken := "Bearer " + user.SecretToken
	if token != expectedToken {
		log.Printf("认证失败: 用户 '%s' 的令牌不匹配。", req.Username)
		http.Error(w, "认证失败: 令牌错误", http.StatusUnauthorized)
		return
	}

	log.Printf("用户 '%s' 认证成功。请求更新IP为: %s", user.Username, req.NewIP)

	// 3. Perform DNS update
	client, err := createAliyunClient()
	if err != nil {
		log.Printf("错误: 创建阿里云客户端失败: %v", err)
		http.Error(w, "服务端配置错误", http.StatusInternalServerError)
		return
	}

	recordID, currentIP, err := findDomainRecordID(client, user.DomainName, user.RR)
	if err != nil {
		log.Printf("错误: 用户 '%s' 查找域名记录失败: %v", user.Username, err)
		http.Error(w, fmt.Sprintf("查找域名记录失败: %v", err), http.StatusInternalServerError)
		return
	}

	if currentIP != nil && *currentIP == req.NewIP {
		log.Printf("用户 '%s' 的IP地址未变化 (%s)，无需更新。", user.Username, req.NewIP)
		fmt.Fprintf(w, `{"status": "success", "message": "IP 地址未变化，无需更新"}`)
		return
	}

	err = updateDomainRecord(client, *recordID, user.RR, req.NewIP)
	if err != nil {
		log.Printf("错误: 用户 '%s' 更新域名记录失败: %v", user.Username, err)
		http.Error(w, fmt.Sprintf("更新域名记录失败: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("成功: 用户 '%s' 的域名 %s.%s 已更新为 %s", user.Username, user.RR, user.DomainName, req.NewIP)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "success", "message": "DNS 记录更新成功"}`)
}

func main() {
	log.Println("DDNS 服务端 (V2.0 多用户版) 启动中...")

	// Initial load
	if err := loadUsers(); err != nil {
		log.Fatalf("错误: 启动时加载用户配置失败: %v", err)
	}

	// You could implement a file watcher to reload the config on change,
	// but for simplicity, we'll just load on startup.

	http.HandleFunc("/update-dns", handleUpdateDNS)

	server := &http.Server{
		Addr:         ":" + listenPort,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	log.Printf("将在端口 %s 上监听请求", listenPort)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("错误: 启动 HTTP 服务器失败: %v", err)
	}
}
