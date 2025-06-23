// ===================================================================================
// File: ddns-client/cmd/key.go
// Description: (新增文件) 负责执行密钥管理命令。
// ===================================================================================
package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/keepsea/goddns/ddns_client/api"
	"github.com/keepsea/goddns/ddns_client/config"
	"github.com/keepsea/goddns/ddns_client/util"
)

type keyViewRequest struct {
	SecretToken string `json:"secret_token"`
}

type keyResetRequest struct {
	SecretToken      string `json:"secret_token"`
	NewEncryptionKey string `json:"new_encryption_key"`
}

func RunViewKey() {
	log.Println("正在向服务端查询您的加密密钥...")
	req, err := http.NewRequest(http.MethodGet, config.App.ServerURL+"/manage-key?username="+config.App.Username, nil)
	if err != nil {
		log.Fatalf("错误: 创建请求失败: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+config.App.SecretToken)
	body, err := api.SendRequest(req)
	if err != nil {
		log.Fatalf("错误: %v", err)
	}
	var resp struct {
		EncryptionKey string `json:"encryption_key"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		log.Fatalf("错误: 解析服务端响应失败: %v", err)
	}
	fmt.Printf("您当前的加密密钥是: %s\n", resp.EncryptionKey)
}

func RunResetKey() {
	log.Println("正在为您生成新的加密密钥...")
	newKey, err := util.GenerateRandomKey()
	if err != nil {
		log.Fatalf("错误: 生成新密钥失败: %v", err)
	}
	log.Printf("新密钥已生成。准备向服务端请求更新...")
	payload := keyResetRequest{
		SecretToken:      config.App.SecretToken,
		NewEncryptionKey: newKey,
	}
	_, err = api.SendSecureRequest("/manage-key", http.MethodPost, payload)
	if err != nil {
		log.Fatalf("错误: 重置密钥失败: %v", err)
	}
	if err := config.SaveKey(newKey); err != nil {
		log.Fatalf("成功重置密钥，但无法自动更新config.ini文件: %v\n请手动将新密钥写入文件:\n%s", err, newKey)
	}
	log.Println("成功: 您的加密密钥已重置，并且config.ini文件已自动更新！")
}
