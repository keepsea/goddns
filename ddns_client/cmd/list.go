// ===================================================================================
// File: ddns-client/cmd/list.go
// Description: 负责执行 'list' 命令。
// ===================================================================================
package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/keepsea/goddns/ddns_client/api"
	"github.com/keepsea/goddns/ddns_client/config"
)

func RunList() {
	log.Println("正在向服务端查询已注册的域名列表...")
	req, err := http.NewRequest(http.MethodGet, config.App.ServerURL+"/manage-records?username="+config.App.Username, nil)
	if err != nil {
		log.Fatalf("错误: 创建请求失败: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.App.SecretToken)
	body, err := api.SendRequest(req)
	if err != nil {
		log.Fatalf("错误: %v", err)
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
