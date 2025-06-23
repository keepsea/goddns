// ===================================================================================
// File: ddns-server/handler/update.go
// Description: 实现 HandleUpdateDNS 函数，专门处理客户端的IP更新请求。它会调用 common.go 的认证函数，然后协调 security 模块进行输入验证，并调用 aliyun 和 config 模块来完成最终的DNS记录创建和更新。
// ===================================================================================
package handler

import (
	"fmt"
	"log"
	"net/http"

	"github.com/keepsea/goddns/ddns_server/aliyun"
	"github.com/keepsea/goddns/ddns_server/config"
	"github.com/keepsea/goddns/ddns_server/security"
)

type UpdateRequest struct {
	SecretToken string `json:"secret_token"`
	DomainName  string `json:"domain_name"`
	RR          string `json:"rr"`
	NewIP       string `json:"new_ip"`
}

func HandleUpdateDNS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST 方法", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateRequest
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
	if err := security.ValidateIPv4(req.NewIP); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, err := aliyun.CreateClient()
	if err != nil {
		log.Printf("错误: 创建阿里云客户端失败: %v", err)
		http.Error(w, "服务端配置错误", http.StatusInternalServerError)
		return
	}

	recordID, currentIP, err := aliyun.GetOrCreateDomainRecord(client, req.DomainName, req.RR, req.NewIP)
	if err != nil {
		log.Printf("错误: 用户 '%s' 获取/创建域名记录失败: %v", username, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := config.BindRecordToUser(username, req.DomainName, req.RR, recordID); err != nil {
		log.Printf("错误: 用户 '%s' 的域名绑定失败: %v", username, err)
		if currentIP != req.NewIP { // Only rollback if we created a new record
			log.Printf("回滚操作：正在删除刚刚为用户 '%s' 创建的记录 %s", username, recordID)
			if delErr := aliyun.DeleteDomainRecord(client, recordID); delErr != nil {
				log.Printf("严重警告：回滚删除操作失败！RecordID: %s, 错误: %v", recordID, delErr)
			}
		}
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	if currentIP == req.NewIP {
		msg := fmt.Sprintf("IP 地址未变化 (%s)，无需更新。", req.NewIP)
		log.Printf("用户 '%s': %s", username, msg)
		fmt.Fprintf(w, `{"status": "success", "message": "%s"}`, msg)
		return
	}

	err = aliyun.UpdateRecordValue(client, recordID, req.RR, req.NewIP)
	if err != nil {
		log.Printf("错误: 用户 '%s' 更新域名记录失败: %v", username, err)
		http.Error(w, fmt.Sprintf("更新域名记录失败: %v", err), http.StatusInternalServerError)
		return
	}

	msg := fmt.Sprintf("域名 %s.%s 已更新为 %s", req.RR, req.DomainName, req.NewIP)
	log.Printf("成功: 用户 '%s' %s", username, msg)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status": "success", "message": "%s"}`, msg)
}
