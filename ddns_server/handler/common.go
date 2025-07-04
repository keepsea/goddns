// ===================================================================================
// File: ddns-server/handler/common.go
// Description: 存放多个处理器都需要用到的通用逻辑，最核心的是 AuthenticateAndDecrypt 函数。这个函数封装了“识别用户 -> 查找密钥 -> 解密数据 -> 认证令牌”这一整套安全流程，极大地简化了其他处理器的代码。
// ===================================================================================
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/keepsea/goddns/ddns_server/config"
	"github.com/keepsea/goddns/ddns_server/security"
)

type BaseRequest struct {
	Username string `json:"username"`
	Data     string `json:"data"`
}

type AuthenticatedRequest struct {
	SecretToken string `json:"secret_token"`
}

func AuthenticateAndDecrypt(r *http.Request, targetStruct interface{}) (string, error) {
	var baseReq BaseRequest
	if err := json.NewDecoder(r.Body).Decode(&baseReq); err != nil {
		return "", fmt.Errorf("请求体JSON格式错误或大小超限")
	}

	if err := security.ValidateUsername(baseReq.Username); err != nil {
		return "", err
	}

	user, ok := config.GetUserByKeyLookup(baseReq.Username)
	if !ok {
		return baseReq.Username, fmt.Errorf("认证失败: 用户 '%s' 不存在", baseReq.Username)
	}

	decryptedPayload, err := security.Decrypt([]byte(user.EncryptionKey), baseReq.Data)
	if err != nil {
		return baseReq.Username, fmt.Errorf("请求解密失败")
	}

	if err := json.Unmarshal(decryptedPayload, targetStruct); err != nil {
		return baseReq.Username, fmt.Errorf("解密后的数据格式错误")
	}

	// Use reflection to get the secret token for authentication
	v := reflect.ValueOf(targetStruct).Elem().FieldByName("SecretToken")
	if !v.IsValid() {
		return baseReq.Username, fmt.Errorf("载荷中缺少SecretToken字段")
	}

	if user.SecretToken != v.String() {
		return baseReq.Username, fmt.Errorf("认证失败: SecretToken不匹配")
	}

	return baseReq.Username, nil
}
