package main

import (
	"embed"
	"fmt"
	"os"

	"singbox-manager/internal/cli"
	"singbox-manager/internal/server"
)

// 前端构建产物随二进制分发：部署 = 复制一个文件。
// web/dist 下保留占位 index.html，未执行 npm run build 时也能 go build。
//
//go:embed all:web/dist
var webFS embed.FS

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "serve":
		// 守护进程（systemd 服务入口）
		if err := server.RunDaemon(webFS); err != nil {
			fmt.Fprintf(os.Stderr, "singbox-manager serve 失败: %v\n", err)
			os.Exit(1)
		}
	case "status":
		if err := cli.Status(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "status 失败: %v\n", err)
			os.Exit(1)
		}
	case "config":
		if len(args) < 2 || args[1] != "sync" {
			usage()
			os.Exit(2)
		}
		if err := cli.ConfigSync(); err != nil {
			fmt.Fprintf(os.Stderr, "config sync 失败: %v\n", err)
			os.Exit(1)
		}
	default:
		// 模式操作：singbox-manager <mode> start|stop
		if len(args) < 2 {
			usage()
			os.Exit(2)
		}
		if err := cli.ModeOp(args[0], args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "%s %s 失败: %v\n", args[0], args[1], err)
			os.Exit(1)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `singbox-manager — Singbox 运行模式管理

用法:
  singbox-manager serve                      启动守护进程（systemd 服务入口）
  singbox-manager <mode> start|stop          启停模式（tun|socks|tproxy|redir-tproxy）
  singbox-manager status [--json]            展示各模式/单元/规则状态
  singbox-manager config sync                同步各模式配置文件

提示: 模式操作与 config sync 均经本机守护进程执行，请先确保
      systemctl start singbox-manager 已运行。
`)
}
