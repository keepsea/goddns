// ===================================================================================
// File: ddns-server/config/config.go
// Description: 负责所有配置的加载、保存和管理。
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

// Server-specific configurations
var (
	ServerPort string
)

const (
	ServerConfigFile = "server.ini"
	UsersConfigFile  = "users.json"
)

// DomainRecord 存储用户的单个域名记录信息。
type DomainRecord struct {
	DomainName string `json:"domain_name"`
	RR         string `json:"rr"`
	RecordID   string `json:"record_id"` // 存储阿里云返回的记录ID，方便删除
}

// User 定义单个用户的配置结构。
type User struct {
	Username    string         `json:"username"`
	SecretToken string         `json:"secret_token"`
	DomainLimit int            `json:"domain_limit"` // 用户可拥有的最大域名数量
	Records     []DomainRecord `json:"records"`      // 用户已注册的域名列表
}

// UserConfig 用于解析整个users.json文件的结构。
type UserConfig struct {
	Users []*User `json:"users"`
}

var (
	userMap      map[string]*User  // 用于快速查找用户的map
	userMapMutex = &sync.RWMutex{} // 读写锁，保证并发安全
)

// LoadServerConfig loads server-specific settings from server.ini
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

// LoadUsers 从 users.json 加载所有用户配置。
func LoadUsers() error {
	userMapMutex.Lock()
	defer userMapMutex.Unlock()

	file, err := os.ReadFile(UsersConfigFile)
	if err != nil {
		// 如果配置文件不存在，则创建一个空的，方便用户首次使用
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

	// 重置并加载数据
	userMap = make(map[string]*User)
	domainRegistry := make(map[string]string) // 用于检查全局域名冲突

	for _, user := range userConfig.Users {
		if user.Username == "" || user.SecretToken == "" {
			log.Printf("警告: 发现配置不完整的用户，已跳过。")
			continue
		}
		// 如果未设置或设置无效，则默认域名额度为1
		if user.DomainLimit <= 0 {
			user.DomainLimit = 1
		}
		userMap[user.Username] = user

		// 检查已注册的域名是否存在冲突
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

// GetUser 安全地获取一个用户的只读副本。
func GetUser(username string) (User, bool) {
	userMapMutex.RLock()
	defer userMapMutex.RUnlock()
	user, ok := userMap[username]
	if !ok {
		return User{}, false
	}
	return *user, true
}

// BindRecordToUser 为用户绑定一条新的域名记录，包含额度和冲突检查。
func BindRecordToUser(username, domainName, rr, recordID string) error {
	userMapMutex.Lock()
	defer userMapMutex.Unlock()

	user, ok := userMap[username]
	if !ok {
		return fmt.Errorf("找不到用户 '%s' 无法绑定记录", username)
	}

	// 检查该用户是否已拥有此域名
	for i := range user.Records {
		if user.Records[i].DomainName == domainName && user.Records[i].RR == rr {
			// 如果记录已存在，只更新阿里云返回的RecordID即可
			user.Records[i].RecordID = recordID
			return saveUsersToFile()
		}
	}

	// 如果是新域名，检查是否达到额度上限
	if len(user.Records) >= user.DomainLimit {
		return fmt.Errorf("域名数量达到上限 (%d)，无法为用户 '%s' 添加新域名", user.DomainLimit, username)
	}

	// 检查新域名是否已被其他任何用户占用
	fullDomain := fmt.Sprintf("%s.%s", rr, domainName)
	for _, u := range userMap {
		if u.Username == username { // Skip self
			continue
		}
		for _, r := range u.Records {
			if r.DomainName == domainName && r.RR == rr {
				return fmt.Errorf("域名 %s 已被用户 '%s' 占用", fullDomain, u.Username)
			}
		}
	}

	// 添加新域名记录并保存
	user.Records = append(user.Records, DomainRecord{DomainName: domainName, RR: rr, RecordID: recordID})
	return saveUsersToFile()
}

// UnbindRecordFromUser 从用户的记录列表中移除一个域名。
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

// saveUsersToFile 将当前用户数据写回 users.json 文件。
func saveUsersToFile() error {
	var userConfig UserConfig
	for _, user := range userMap {
		userConfig.Users = append(userConfig.Users, user)
	}

	file, err := json.MarshalIndent(userConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化用户配置失败: %w", err)
	}

	return os.WriteFile(UsersConfigFile, file, 0644)
}
