// ===================================================================================
// File: ddns-server/config/config.go
// Description:  项目的数据和配置管理中心。
// 功能:
// - 定义 User, DomainRecord 等核心数据结构。
// - 从 server.ini 加载服务自身配置（如端口号）。
// - 从 users.json 加载、解析所有用户信息，并将其存入一个易于查询的map中。
// - 提供线程安全的函数（如 GetUserByKeyLookup, BindRecordToUser, UnbindRecordFromUser, UpdateUserKey）来增、删、改、查用户数据。
// - 在用户注册新域名时，进行额度检查和全局域名冲突检查。
// - 负责将更新后的用户数据写回 users.json 文件，实现数据持久化。
//
// ===================================================================================
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"gopkg.in/ini.v1"
)

var (
	ServerPort string
)

const (
	ServerConfigFile = "server.ini"
	UsersConfigFile  = "users.json"
)

type DomainRecord struct {
	DomainName string `json:"domain_name"`
	RR         string `json:"rr"`
	RecordID   string `json:"record_id"`
}

type User struct {
	Username      string         `json:"username"`
	SecretToken   string         `json:"secret_token"`
	EncryptionKey string         `json:"encryption_key"`
	DomainLimit   int            `json:"domain_limit"`
	Records       []DomainRecord `json:"records"`
}

type UserConfig struct {
	Users []*User `json:"users"`
}

var (
	userMap      map[string]*User
	userMapMutex = &sync.RWMutex{}
)

func LoadServerConfig() error {
	cfg, err := ini.Load(ServerConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("警告: 找不到 %s，将使用默认端口 9876。", ServerConfigFile)
			ServerPort = "9876"
			return nil
		}
		return fmt.Errorf("无法加载服务端配置文件 %s: %w", ServerConfigFile, err)
	}
	serverSection := cfg.Section("server")
	ServerPort = serverSection.Key("listen_port").MustString("9876")
	return nil
}

func LoadUsers() error {
	userMapMutex.Lock()
	defer userMapMutex.Unlock()

	file, err := os.ReadFile(UsersConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("警告: 找不到 %s，将创建一个空的配置文件。", UsersConfigFile)
			userMap = make(map[string]*User)
			return saveUsersToFile()
		}
		return fmt.Errorf("无法读取用户配置文件 %s: %w", UsersConfigFile, err)
	}

	var userConfig UserConfig
	if len(file) > 0 {
		if err := json.Unmarshal(file, &userConfig); err != nil {
			return fmt.Errorf("解析用户配置文件JSON失败: %w", err)
		}
	}

	userMap = make(map[string]*User)
	domainRegistry := make(map[string]string)

	for _, user := range userConfig.Users {
		if user.Username == "" || user.SecretToken == "" || len(user.EncryptionKey) != 32 {
			log.Printf("警告: 用户 '%s' 的配置不完整或encryption_key长度不为32，已跳过。", user.Username)
			continue
		}
		if user.DomainLimit <= 0 {
			user.DomainLimit = 1
		}
		userMap[user.Username] = user
		for _, record := range user.Records {
			fullDomain := fmt.Sprintf("%s.%s", record.RR, record.DomainName)
			if owner, exists := domainRegistry[fullDomain]; exists {
				return fmt.Errorf("域名冲突: %s 已被用户 '%s' 注册", fullDomain, owner)
			}
			domainRegistry[fullDomain] = user.Username
		}
	}
	log.Printf("成功加载 %d 个用户配置。", len(userMap))
	return nil
}

func GetUserByKeyLookup(username string) (User, bool) {
	userMapMutex.RLock()
	defer userMapMutex.RUnlock()
	user, ok := userMap[username]
	if !ok {
		return User{}, false
	}
	return *user, true
}

func BindRecordToUser(username, domainName, rr, recordID string) error {
	userMapMutex.Lock()
	defer userMapMutex.Unlock()
	user, ok := userMap[username]
	if !ok {
		return fmt.Errorf("找不到用户 '%s' 无法绑定记录", username)
	}
	for i := range user.Records {
		if user.Records[i].DomainName == domainName && user.Records[i].RR == rr {
			user.Records[i].RecordID = recordID
			return saveUsersToFile()
		}
	}
	if len(user.Records) >= user.DomainLimit {
		return fmt.Errorf("域名数量达到上限 (%d)，无法为用户 '%s' 添加新域名", user.DomainLimit, username)
	}
	fullDomain := fmt.Sprintf("%s.%s", rr, domainName)
	for _, u := range userMap {
		if u.Username == username {
			continue
		}
		for _, r := range u.Records {
			if r.DomainName == domainName && r.RR == rr {
				return fmt.Errorf("域名 %s 已被用户 '%s' 占用", fullDomain, u.Username)
			}
		}
	}
	user.Records = append(user.Records, DomainRecord{DomainName: domainName, RR: rr, RecordID: recordID})
	return saveUsersToFile()
}

func UnbindRecordFromUser(username, domainName, rr string) (string, error) {
	userMapMutex.Lock()
	defer userMapMutex.Unlock()
	user, ok := userMap[username]
	if !ok {
		return "", fmt.Errorf("找不到用户 '%s' 无法注销域名", username)
	}
	var recordID string
	found := false
	var newRecords []DomainRecord
	for _, record := range user.Records {
		if record.DomainName == domainName && record.RR == rr {
			recordID = record.RecordID
			found = true
		} else {
			newRecords = append(newRecords, record)
		}
	}
	if !found {
		return "", fmt.Errorf("用户 '%s' 名下未找到域名 %s.%s", username, rr, domainName)
	}
	user.Records = newRecords
	return recordID, saveUsersToFile()
}

func UpdateUserKey(username, newKey string) error {
	userMapMutex.Lock()
	defer userMapMutex.Unlock()
	user, ok := userMap[username]
	if !ok {
		return fmt.Errorf("找不到用户 '%s'", username)
	}
	if len(newKey) != 32 {
		return fmt.Errorf("新密钥长度必须为32个字符")
	}
	user.EncryptionKey = newKey
	return saveUsersToFile()
}

func saveUsersToFile() error {
	var userConfig UserConfig
	userList := make([]*User, 0, len(userMap))
	for _, user := range userMap {
		userList = append(userList, user)
	}
	userConfig.Users = userList
	file, err := json.MarshalIndent(userConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化用户配置失败: %w", err)
	}

	tmpFile := UsersConfigFile + ".tmp"
	if err := os.WriteFile(tmpFile, file, 0600); err != nil {
		return fmt.Errorf("写入临时用户配置文件失败: %w", err)
	}
	if err := os.Rename(tmpFile, UsersConfigFile); err != nil {
		return fmt.Errorf("原子重命名用户配置文件失败: %w", err)
	}
	return nil
}
