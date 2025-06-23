// ===================================================================================
// File: ddns-client/cmd/remove.go
// Description: (新增文件) 负责执行 'remove' 命令。
// ===================================================================================
package cmd

import (
	"log"
	"net/http"
	"strings"

	"github.com/keepsea/goddns/ddns_client/api"
	"github.com/keepsea/goddns/ddns_client/config"
)

type manageRequest struct {
	SecretToken string `json:"secret_token"`
	DomainName  string `json:"domain_name"`
	RR          string `json:"rr"`
}

func RunRemove(fullDomain string) {
	log.Printf("准备向服务端请求注销域名: %s", fullDomain)
	parts := strings.SplitN(fullDomain, ".", 2)
	if len(parts) < 2 {
		log.Fatalf("域名格式错误。请输入完整域名，例如 'home.example.com'")
	}
	rr, domainName := parts[0], parts[1]
	payload := manageRequest{
		SecretToken: config.App.SecretToken,
		DomainName:  domainName,
		RR:          rr,
	}
	body, err := api.SendSecureRequest("/manage-records", http.MethodDelete, payload)
	if err != nil {
		log.Fatalf("错误: %v", err)
	}
	log.Printf("成功: 服务端响应: %s", string(body))
}
