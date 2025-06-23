// ===================================================================================
// File: ddns-server/security/validator.go
// ===================================================================================
package security

import (
	"fmt"
	"net"
	"regexp"
)

var (
	domainPartRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
	rrRegex         = regexp.MustCompile(`^@$|^[a-zA-Z0-9*]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
	usernameRegex   = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,20}$`)
)

func ValidateDomain(domain string) error {
	parts := regexp.MustCompile(`\.`).Split(domain, -1)
	if len(parts) < 2 {
		return fmt.Errorf("域名格式无效: '%s'，至少需要一个点", domain)
	}
	for _, part := range parts {
		if !domainPartRegex.MatchString(part) {
			return fmt.Errorf("域名部分 '%s' 包含无效字符或格式", part)
		}
	}
	return nil
}
func ValidateRR(rr string) error {
	if !rrRegex.MatchString(rr) {
		return fmt.Errorf("主机记录 (RR) '%s' 包含无效字符或格式", rr)
	}
	return nil
}
func ValidateIPv4(ip string) error {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil || parsedIP.To4() == nil {
		return fmt.Errorf("IP地址 '%s' 不是一个有效的IPv4地址", ip)
	}
	return nil
}
func ValidateUsername(username string) error {
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("用户名 '%s' 包含无效字符或长度不符合要求(3-20位)", username)
	}
	return nil
}
