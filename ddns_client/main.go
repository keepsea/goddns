// ===================================================================================
// File: ddns-client/main.go
// Description: 项目主入口，负责解析命令行标志并分发任务。
// ===================================================================================
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/keepsea/goddns/ddns_client/cmd"
	"github.com/keepsea/goddns/ddns_client/config"
)

func main() {
	log.SetFlags(0)

	updateFlag := flag.Bool("update", false, "启动后台守护进程，持续更新IP地址 (默认操作)。")
	listFlag := flag.Bool("list", false, "查询并列出当前用户已注册的所有域名。")
	removeFlag := flag.String("remove", "", "注销一个已注册的域名。用法: -remove <rr.domain.com>")
	viewKeyFlag := flag.Bool("view-key", false, "查询并显示您当前的加密密钥。")
	resetKeyFlag := flag.Bool("reset-key", false, "生成一个新密钥并向服务端请求重置。")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "goddns-client: 一个多功能DDNS客户端工具。\n\n")
		fmt.Fprintf(os.Stderr, "使用方法:\n")
		fmt.Fprintf(os.Stderr, "  goddns-client [flags]\n\n")
		fmt.Fprintf(os.Stderr, "默认操作 (不带任何标志) 是启动IP更新守护进程。\n\n")
		fmt.Fprintf(os.Stderr, "可用标志:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// 根据标志决定执行哪个命令
	if *listFlag {
		if err := config.Load(false); err != nil {
			log.Fatalf("错误: %v", err)
		}
		cmd.RunList()
	} else if *removeFlag != "" {
		if err := config.Load(false); err != nil {
			log.Fatalf("错误: %v", err)
		}
		cmd.RunRemove(*removeFlag)
	} else if *viewKeyFlag {
		if err := config.Load(false); err != nil {
			log.Fatalf("错误: %v", err)
		}
		cmd.RunViewKey()
	} else if *resetKeyFlag {
		if err := config.Load(false); err != nil {
			log.Fatalf("错误: %v", err)
		}
		cmd.RunResetKey()
	} else {
		*updateFlag = true
		if err := config.Load(true); err != nil {
			log.Fatalf("错误: %v", err)
		}
		cmd.RunUpdateDaemon()
	}
}
