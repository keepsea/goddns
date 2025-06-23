// ===================================================================================
// File: ddns-server/handler/update.go
// Description: 负责处理所有与DNS记录更新相关的HTTP请求。
// ===================================================================================
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/keepsea/goddns/ddns_server/aliyun"
	"github.com/keepsea/goddns/ddns_server/config"
)

// UpdateRequest 是客户端发来的更新IP请求的结构体。
type UpdateRequest struct {
	Username   string `json:"username"`
	DomainName string `json:"domain_name"`
	RR         string `json:"rr"`
	NewIP      string `json:"new_ip"`
}

// authUser 是一个辅助函数，用于验证用户身份。
func authUser(r *http.Request, username string) (config.User, error) {
	user, ok := config.GetUser(username)
	if !ok {
		return config.User{}, fmt.Errorf("认证失败: 用户 '%s' 不存在", username)
	}
	token := r.Header.Get("Authorization")
	if token != "Bearer "+user.SecretToken {
		return config.User{}, fmt.Errorf("认证失败: 用户 '%s' 的令牌不匹配", username)
	}
	return user, nil
}

// HandleUpdateDNS 处理IP更新请求的核心逻辑。
func HandleUpdateDNS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "无法读取请求体", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "请求体 JSON 格式错误", http.StatusBadRequest)
		return
	}

	// 1. 验证用户身份
	_, err = authUser(r, req.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		log.Print(err)
		return
	}

	// 2. 获取或创建域名记录
	client, err := aliyun.CreateClient()
	if err != nil {
		log.Printf("错误: 创建阿里云客户端失败: %v", err)
		http.Error(w, "服务端配置错误", http.StatusInternalServerError)
		return
	}

	recordID, currentIP, err := aliyun.GetOrCreateDomainRecord(client, req.DomainName, req.RR, req.NewIP)
	if err != nil {
		log.Printf("错误: 用户 '%s' 获取/创建域名记录失败: %v", req.Username, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. 将记录ID与用户绑定（包含额度检查和冲突检查）
	if err := config.BindRecordToUser(req.Username, req.DomainName, req.RR, recordID); err != nil {
		log.Printf("错误: 用户 '%s' 的域名绑定失败: %v", req.Username, err)
		// 如果绑定失败（例如达到限额），需要删除刚刚可能创建的记录
		log.Printf("回滚操作：正在删除刚刚为用户 '%s' 创建的记录 %s", req.Username, recordID)
		if delErr := aliyun.DeleteDomainRecord(client, recordID); delErr != nil {
			log.Printf("严重警告：回滚删除操作失败！RecordID: %s, 错误: %v", recordID, delErr)
		}
		http.Error(w, err.Error(), http.StatusConflict) // 409 Conflict
		return
	}

	// 4. 对比并更新IP
	if currentIP == req.NewIP {
		msg := fmt.Sprintf("IP 地址未变化 (%s)，无需更新。", req.NewIP)
		log.Printf("用户 '%s': %s", req.Username, msg)
		fmt.Fprintf(w, `{"status": "success", "message": "%s"}`, msg)
		return
	}

	err = aliyun.UpdateRecordValue(client, recordID, req.RR, req.NewIP)
	if err != nil {
		log.Printf("错误: 用户 '%s' 更新域名记录失败: %v", req.Username, err)
		http.Error(w, fmt.Sprintf("更新域名记录失败: %v", err), http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("域名 %s.%s 已更新为 %s", req.RR, req.DomainName, req.NewIP)
	log.Printf("成功: 用户 '%s' %s", req.Username, msg)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "success", "message": "%s"}`, msg)
}
