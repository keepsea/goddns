// ===================================================================================
// File: ddns-client/api/client.go
// Description: 封装所有与服务端的API交互。
// ===================================================================================
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/keepsea/goddns/ddns_client/config"
	"github.com/keepsea/goddns/ddns_client/security"
)

type BaseRequest struct {
	Username string `json:"username"`
	Data     string `json:"data"`
}

func SendSecureRequest(endpoint string, method string, payload interface{}) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化载荷失败: %w", err)
	}
	encryptedData, err := security.Encrypt([]byte(config.App.EncryptionKey), payloadBytes)
	if err != nil {
		return nil, fmt.Errorf("加密载荷失败: %w", err)
	}
	finalRequest := BaseRequest{Username: config.App.Username, Data: encryptedData}
	finalRequestBytes, _ := json.Marshal(finalRequest)
	req, err := http.NewRequest(method, config.App.ServerURL+endpoint, bytes.NewBuffer(finalRequestBytes))
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return SendRequest(req)
}

// SendRequest 是一个用于发送简单、无需加密请求体（如GET）的辅助函数。
func SendRequest(req *http.Request) ([]byte, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("服务端返回错误 (状态码: %d): %s", resp.StatusCode, string(body))
	}
	return body, nil
}
