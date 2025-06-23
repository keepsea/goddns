// ===================================================================================
// File: ddns-server/aliyun/dns.go
// Description: 封装所有与阿里云DNS API的交互。
// ===================================================================================
package aliyun

import (
	"fmt"

	alidns20150109 "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/credentials-go/credentials"
)

// CreateClient 创建并返回一个阿里云DNS客户端实例。
func CreateClient() (*alidns20150109.Client, error) {
	cred, err := credentials.NewCredential(nil)
	if err != nil {
		return nil, err
	}
	config := &openapi.Config{
		Credential: cred,
		Endpoint:   tea.String("dns.aliyuncs.com"),
	}
	return alidns20150109.NewClient(config)
}

// getDomainRecordInternal 内部函数，用于查找一个具体的域名记录。
func getDomainRecordInternal(client *alidns20150109.Client, domainName, rr string) (*alidns20150109.DescribeDomainRecordsResponseBodyDomainRecordsRecord, error) {
	req := &alidns20150109.DescribeDomainRecordsRequest{
		DomainName: tea.String(domainName),
		RRKeyWord:  tea.String(rr),
		Type:       tea.String("A"),
	}
	resp, err := client.DescribeDomainRecords(req)
	if err != nil {
		return nil, err
	}
	// 精确匹配主机记录 (RR)
	for _, record := range resp.Body.DomainRecords.Record {
		if *record.RR == rr {
			return record, nil
		}
	}
	return nil, nil // 未找到，但不是错误
}

// addDomainRecord 创建一条新的A记录。
func addDomainRecord(client *alidns20150109.Client, domainName, rr, ip string) (*string, error) {
	req := &alidns20150109.AddDomainRecordRequest{
		DomainName: tea.String(domainName),
		RR:         tea.String(rr),
		Type:       tea.String("A"),
		Value:      tea.String(ip),
	}
	resp, err := client.AddDomainRecord(req)
	if err != nil {
		return nil, err
	}
	return resp.Body.RecordId, nil
}

// UpdateRecordValue 为一个已存在的记录更新IP地址。
func UpdateRecordValue(client *alidns20150109.Client, recordId, rr, newIP string) error {
	req := &alidns20150109.UpdateDomainRecordRequest{
		RecordId: tea.String(recordId),
		RR:       tea.String(rr),
		Type:     tea.String("A"),
		Value:    tea.String(newIP),
	}
	_, err := client.UpdateDomainRecord(req)
	return err
}

// GetOrCreateDomainRecord 查找一条记录，如果不存在则创建它。
// 返回记录的ID和它当前的IP值。
func GetOrCreateDomainRecord(client *alidns20150109.Client, domainName, rr, ip string) (string, string, error) {
	record, err := getDomainRecordInternal(client, domainName, rr)
	if err != nil {
		return "", "", fmt.Errorf("查找域名记录时出错: %w", err)
	}

	// 如果记录不存在，则创建它
	if record == nil {
		recordId, err := addDomainRecord(client, domainName, rr, ip)
		if err != nil {
			return "", "", fmt.Errorf("创建新域名记录时出错: %w", err)
		}
		// 因为是新创建的，所以当前值就是我们刚刚设置的IP
		return *recordId, ip, nil
	}

	// 如果记录已存在，返回它的ID和当前值
	return *record.RecordId, *record.Value, nil
}

// DeleteDomainRecord 从阿里云删除一条DNS记录。
func DeleteDomainRecord(client *alidns20150109.Client, recordId string) error {
	req := &alidns20150109.DeleteDomainRecordRequest{
		RecordId: tea.String(recordId),
	}
	_, err := client.DeleteDomainRecord(req)
	return err
}
