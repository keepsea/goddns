// ===================================================================================
// File: ddns-server/handler/key.go
// ===================================================================================
package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/keepsea/goddns/ddns_server/config"
	"github.com/keepsea/goddns/ddns_server/security"
)

type KeyViewResponse struct {
	EncryptionKey string `json:"encryption_key"`
}

type KeyResetRequest struct {
	SecretToken      string `json:"secret_token"`
	NewEncryptionKey string `json:"new_encryption_key"`
}

func HandleManageKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "仅支持 GET 和 POST 方法", http.StatusMethodNotAllowed)
		return
	}
	if r.Method == http.MethodGet {
		handleViewKey(w, r)
	} else {
		handleResetKey(w, r)
	}
}

func handleViewKey(w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	if err := security.ValidateUsername(username); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
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

	resp := KeyViewResponse{EncryptionKey: user.EncryptionKey}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("错误: 序列化用户 '%s' 的密钥失败: %v", user.Username, err)
		http.Error(w, "内部服务器错误", http.StatusInternalServerError)
	}
	log.Printf("用户 '%s' 查询了其加密密钥。", user.Username)
}

func handleResetKey(w http.ResponseWriter, r *http.Request) {
	var req KeyResetRequest
	username, err := AuthenticateAndDecrypt(r, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		log.Printf("请求处理失败 (用户: %s): %v", username, err)
		return
	}

	if len(req.NewEncryptionKey) != 32 {
		http.Error(w, "新密钥长度必须为32个字符", http.StatusBadRequest)
		return
	}

	if err := config.UpdateUserKey(username, req.NewEncryptionKey); err != nil {
		log.Printf("错误: 用户 '%s' 更新加密密钥失败: %v", username, err)
		http.Error(w, "更新密钥时发生内部错误", http.StatusInternalServerError)
		return
	}

	msg := "加密密钥已成功重置。"
	log.Printf("成功: 用户 '%s' %s", username, msg)
	fmt.Fprintf(w, `{"status":"success", "message":"%s"}`, msg)
}
