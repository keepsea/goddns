// ===================================================================================
// File: ddns-server/handler/records.go
// Description: 实现 HandleManageRecords 函数，负责处理用户对域名记录的自助管理。它会根据HTTP请求的方法（GET或DELETE），分别调用内部的handleList（查看）或handleDelete（删除）逻辑。
// ===================================================================================
package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/keepsea/goddns/ddns_server/aliyun"
	"github.com/keepsea/goddns/ddns_server/config"
	"github.com/keepsea/goddns/ddns_server/security"
)

type ManageRequest struct {
	SecretToken string `json:"secret_token"`
	DomainName  string `json:"domain_name,omitempty"`
	RR          string `json:"rr,omitempty"`
}

func HandleManageRecords(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		http.Error(w, "仅支持 GET 和 DELETE 方法", http.StatusMethodNotAllowed)
		return
	}

	if r.Method == http.MethodGet {
		handleList(w, r)
	} else {
		handleDelete(w, r)
	}
}

func handleList(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if err := security.ValidateUsername(username); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// For GET, we cannot read the body for token.
	// The auth must be done via header.
	user, ok := config.GetUserByKeyLookup(username)
	if !ok {
		http.Error(w, "认证失败: 用户不存在", http.StatusUnauthorized)
		return
	}
	token := r.Header.Get("Authorization")
	if token != "Bearer "+user.SecretToken {
		http.Error(w, "认证失败: 令牌不匹配", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user.Records); err != nil {
		log.Printf("错误: 序列化用户 '%s' 的记录列表失败: %v", user.Username, err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
	log.Printf("用户 '%s' 查询了其域名列表。", user.Username)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	var req ManageRequest
	username, err := AuthenticateAndDecrypt(r, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		log.Printf("请求处理失败 (用户: %s): %v", username, err)
		return
	}

	if err := security.ValidateDomain(req.DomainName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := security.ValidateRR(req.RR); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	recordID, err := config.UnbindRecordFromUser(username, req.DomainName, req.RR)
	if err != nil {
		log.Printf("错误: 用户 '%s' 注销域名失败: %v", username, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if recordID == "" {
		log.Printf("警告: 用户 '%s' 尝试删除的域名 %s.%s 没有关联的 RecordID，仅从本地配置中移除。", username, req.RR, req.DomainName)
		fmt.Fprintf(w, `{"status":"success", "message":"域名已从配置中移除，但阿里云端无对应记录可删除。"}`)
		return
	}

	client, err := aliyun.CreateClient()
	if err != nil {
		log.Printf("错误: 创建阿里云客户端失败: %v", err)
		http.Error(w, "服务端配置错误", http.StatusInternalServerError)
		return
	}
	if err := aliyun.DeleteDomainRecord(client, recordID); err != nil {
		log.Printf("严重警告: 从配置中移除了用户 '%s' 的域名 %s.%s，但在阿里云删除失败: %v", username, req.RR, req.DomainName, err)
		http.Error(w, fmt.Sprintf("域名已从配置中移除，但在阿里云删除失败: %v", err), http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("域名 %s.%s 已成功注销。", req.RR, req.DomainName)
	log.Printf("成功: 用户 '%s' %s", username, msg)
	fmt.Fprintf(w, `{"status":"success", "message":"%s"}`, msg)
}
