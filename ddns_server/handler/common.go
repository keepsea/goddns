// ===================================================================================
// File: ddns-server/handler/common.go
// Description: 存放通用的请求/响应结构体和认证逻辑。
// ===================================================================================
// ===================================================================================
// File: ddns-server/handler/common.go
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
