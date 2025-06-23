// ===================================================================================
// File: ddns-client/config/config.go
// Description: 负责加载和解析 config.ini 文件。
// ===================================================================================
package config

import (
	"fmt"

	"gopkg.in/ini.v1"
)

var App AppConfig

type AppConfig struct {
	ServerURL            string
	Username             string
	SecretToken          string
	EncryptionKey        string
	DomainName           string
	RR                   string
	CheckIntervalSeconds int
}

func Load(isUpdateDaemon bool) error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		return fmt.Errorf("无法加载 config.ini: %v", err)
	}

	clientSection := cfg.Section("client")
	App.ServerURL = clientSection.Key("server_url").String()
	App.Username = clientSection.Key("username").String()
	App.SecretToken = clientSection.Key("secret_token").String()
	App.EncryptionKey = clientSection.Key("encryption_key").String()

	if App.ServerURL == "" || App.Username == "" || App.SecretToken == "" || App.EncryptionKey == "" {
		return fmt.Errorf("config.ini 中缺少核心配置项 (server_url, username, secret_token, encryption_key)")
	}

	if isUpdateDaemon {
		App.DomainName = clientSection.Key("domain_name").String()
		App.RR = clientSection.Key("rr").String()
		App.CheckIntervalSeconds = clientSection.Key("check_interval_seconds").MustInt(300)
		if App.DomainName == "" || App.RR == "" {
			return fmt.Errorf("config.ini 中缺少 domain_name 或 rr 配置项")
		}
	}
	return nil
}

func SaveKey(newKey string) error {
	cfg, err := ini.Load("config.ini")
	if err != nil {
		return fmt.Errorf("无法加载config.ini以更新密钥: %w", err)
	}
	cfg.Section("client").Key("encryption_key").SetValue(newKey)
	App.EncryptionKey = newKey
	return cfg.SaveTo("config.ini")
}
