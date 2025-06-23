// ===================================================================================
// File: ddns-server/aliyun/dns.go
// Description: 封装所有与阿里云云解析DNS (Alidns) API 的直接交互。
// 功能:
// - 提供 CreateClient() 函数，用于创建与阿里云通信的客户端实例。
// - 实现 GetOrCreateDomainRecord()，封装了“查找或创建A记录”的原子操作。
// - 实现 UpdateRecordValue()，用于更新已有记录的IP地址。
// - 实现 DeleteDomainRecord()，用于删除指定的A记录。
// - 这个模块的存在，使得如果未来想支持其他DNS服务商（如腾讯云DNSPod），我们只需要新增一个类似的模块即可，而无需改动核心业务逻辑。
//
// ===================================================================================
package aliyun

import (
	"fmt"

	alidns20150109 "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/credentials-go/credentials"
)

func CreateClient() (*alidns20150109.Client, error) {
	cred, err := credentials.NewCredential(nil)
	if err != nil {
		return nil, err
	}
	config := &openapi.Config{Credential: cred, Endpoint: tea.String("dns.aliyuncs.com")}
	return alidns20150109.NewClient(config)
}
func getDomainRecordInternal(client *alidns20150109.Client, domainName, rr string) (*alidns20150109.DescribeDomainRecordsResponseBodyDomainRecordsRecord, error) {
	req := &alidns20150109.DescribeDomainRecordsRequest{DomainName: tea.String(domainName), RRKeyWord: tea.String(rr), Type: tea.String("A")}
	resp, err := client.DescribeDomainRecords(req)
	if err != nil {
		return nil, err
	}
	for _, record := range resp.Body.DomainRecords.Record {
		if *record.RR == rr {
			return record, nil
		}
	}
	return nil, nil
}
func addDomainRecord(client *alidns20150109.Client, domainName, rr, ip string) (*string, error) {
	req := &alidns20150109.AddDomainRecordRequest{DomainName: tea.String(domainName), RR: tea.String(rr), Type: tea.String("A"), Value: tea.String(ip)}
	resp, err := client.AddDomainRecord(req)
	if err != nil {
		return nil, err
	}
	return resp.Body.RecordId, nil
}
func UpdateRecordValue(client *alidns20150109.Client, recordId, rr, newIP string) error {
	req := &alidns20150109.UpdateDomainRecordRequest{RecordId: tea.String(recordId), RR: tea.String(rr), Type: tea.String("A"), Value: tea.String(newIP)}
	_, err := client.UpdateDomainRecord(req)
	return err
}
func GetOrCreateDomainRecord(client *alidns20150109.Client, domainName, rr, ip string) (string, string, error) {
	record, err := getDomainRecordInternal(client, domainName, rr)
	if err != nil {
		return "", "", fmt.Errorf("查找域名记录时出错: %w", err)
	}
	if record == nil {
		recordId, err := addDomainRecord(client, domainName, rr, ip)
		if err != nil {
			return "", "", fmt.Errorf("创建新域名记录时出错: %w", err)
		}
		return *recordId, ip, nil
	}
	return *record.RecordId, *record.Value, nil
}
func DeleteDomainRecord(client *alidns20150109.Client, recordId string) error {
	req := &alidns20150109.DeleteDomainRecordRequest{RecordId: tea.String(recordId)}
	_, err := client.DeleteDomainRecord(req)
	return err
}
