package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"text/tabwriter"

	"singbox-manager/internal/cli"
	"singbox-manager/internal/server"
)

// 构建信息：由构建脚本经 -ldflags "-X main.version=... -X main.buildTime=..."
// 注入；直接 go build 时为默认值（dev/unknown）。
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
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
	case "info":
		printInfo()
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

// printInfo 展示构建信息，用于区分二进制是否为最新构建。
func printInfo() {
	fmt.Printf("singbox-manager %s\n\n", version)
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintf(w, "版本:\t%s\n", version)
	fmt.Fprintf(w, "编译时间:\t%s\n", buildTime)
	fmt.Fprintf(w, "Git 提交:\t%s\n", gitCommit)
	fmt.Fprintf(w, "Go 版本:\t%s\n", runtime.Version())
	fmt.Fprintf(w, "平台:\t%s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(w, "内嵌前端:\t%s\n", embeddedFrontend())
	w.Flush()
}

// embeddedFrontend 报告内嵌前端是真实构建还是占位页。
func embeddedFrontend() string {
	entries, err := fs.ReadDir(webFS, "web/dist/assets")
	if err != nil || len(entries) == 0 {
		return "占位页（未构建前端）"
	}
	var size int64
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			size += info.Size()
		}
	}
	return fmt.Sprintf("已构建（%d 个资源，%.1f KB）", len(entries), float64(size)/1024)
}

func usage() {
	fmt.Fprintf(os.Stderr, `singbox-manager — Singbox 运行模式管理

用法:
  singbox-manager serve                      启动守护进程（systemd 服务入口）
  singbox-manager info                       查看版本/编译时间/平台等构建信息
  singbox-manager <mode> start|stop          启停模式（tun|socks|tproxy|redir-tproxy）
  singbox-manager status [--json]            展示各模式/单元/规则状态
  singbox-manager config sync                同步各模式配置文件

提示: 模式操作与 config sync 均经本机守护进程执行，请先确保
      systemctl start singbox-manager 已运行。
`)
}
