// ===================================================================================
// File: ddns-server/handler/records.go
// Description: 负责处理所有与用户域名记录管理（查看/删除）相关的HTTP请求。
// ===================================================================================
package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/keepsea/goddns/ddns_server/aliyun"
	"github.com/keepsea/goddns/ddns_server/config"
)

// ManageRequest 用于解析查看和删除记录的请求。
type ManageRequest struct {
	Username   string `json:"username"`
	DomainName string `json:"domain_name,omitempty"` // 删除时需要
	RR         string `json:"rr,omitempty"`          // 删除时需要
}

// HandleManageRecords 统一处理GET（查看）和DELETE（删除）请求。
func HandleManageRecords(w http.ResponseWriter, r *http.Request) {
	var req ManageRequest

	// 根据请求方法解析请求参数
	if r.Method == http.MethodGet {
		req.Username = r.URL.Query().Get("username")
	} else if r.Method == http.MethodDelete {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "请求体 JSON 格式错误", http.StatusBadRequest)
			return
		}
	} else {
		http.Error(w, "仅支持 GET 和 DELETE 方法", http.StatusMethodNotAllowed)
		return
	}

	// 1. 验证用户身份
	user, err := authUser(r, req.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		log.Print(err)
		return
	}

	// 2. 根据请求方法分发到不同的处理函数
	switch r.Method {
	case http.MethodGet:
		handleList(w, user)
	case http.MethodDelete:
		handleDelete(w, user, req)
	}
}

// handleList 处理查看用户域名列表的请求。
func handleList(w http.ResponseWriter, user config.User) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user.Records); err != nil {
		log.Printf("错误: 序列化用户 '%s' 的记录列表失败: %v", user.Username, err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
	log.Printf("用户 '%s' 查询了其域名列表。", user.Username)
}

// handleDelete 处理删除用户某个域名的请求。
func handleDelete(w http.ResponseWriter, user config.User, req ManageRequest) {
	if req.DomainName == "" || req.RR == "" {
		http.Error(w, "删除请求必须包含 'domain_name' 和 'rr'", http.StatusBadRequest)
		return
	}

	// 1. 从用户配置中解绑域名，并获取其在阿里云的RecordID
	recordID, err := config.UnbindRecordFromUser(user.Username, req.DomainName, req.RR)
	if err != nil {
		log.Printf("错误: 用户 '%s' 注销域名失败: %v", user.Username, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// 如果在我们系统中找到了记录，但它没有RecordID，说明可能从未在阿里云成功创建过
	if recordID == "" {
		log.Printf("警告: 用户 '%s' 尝试删除的域名 %s.%s 没有关联的 RecordID，仅从本地配置中移除。", user.Username, req.RR, req.DomainName)
		fmt.Fprintf(w, `{"status":"success", "message":"域名已从配置中移除，但阿里云端无对应记录可删除。"}`)
		return
	}

	// 2. 从阿里云删除该记录
	client, err := aliyun.CreateClient()
	if err != nil {
		log.Printf("错误: 创建阿里云客户端失败: %v", err)
		http.Error(w, "服务端配置错误", http.StatusInternalServerError)
		return
	}
	if err := aliyun.DeleteDomainRecord(client, recordID); err != nil {
		// 即使在阿里云删除失败，我们的配置也已经移除了该记录，所以只打印警告即可。
		// 这可以防止因阿里云API临时故障导致用户无法释放配额的问题。
		log.Printf("严重警告: 从配置中移除了用户 '%s' 的域名 %s.%s，但在阿里云删除失败: %v", user.Username, req.RR, req.DomainName, err)
		http.Error(w, fmt.Sprintf("域名已从配置中移除，但在阿里云删除失败: %v", err), http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("域名 %s.%s 已成功注销。", req.RR, req.DomainName)
	log.Printf("成功: 用户 '%s' %s", user.Username, msg)
	fmt.Fprintf(w, `{"status":"success", "message":"%s"}`, msg)
}
