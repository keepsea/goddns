// ===================================================================================
// File: ddns-client/util/util.go
// Description: 存放通用的工具函数。
// ===================================================================================
package util

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const lastIPFile = "last_ip.txt"

func GetPublicIP() (string, error) {
	ipServices := []string{"https://api.ipify.org", "http://ifconfig.me/ip"}
	for _, service := range ipServices {
		resp, err := http.Get(service)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ip, err := io.ReadAll(resp.Body)
				if err == nil {
					return strings.TrimSpace(string(ip)), nil
				}
			}
		}
	}
	return "", fmt.Errorf("未能获取公网 IP")
}

func ReadLastIP() (string, error) {
	data, err := os.ReadFile(lastIPFile)
	if os.IsNotExist(err) {
		return "", nil
	}
	return string(data), err
}

func WriteLastIP(ip string) error {
	return os.WriteFile(lastIPFile, []byte(ip), 0644)
}

func GenerateRandomKey() (string, error) {
	key := make([]byte, 24) // 24 bytes = 32 base64 chars
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
