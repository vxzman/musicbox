// Package intercept 通过调用系统 ip / nft 命令实现透明代理的网络接管。
//
// 设计目标：核心无关、零依赖——不 import 任何 singbox 代码，只依赖 iproute2
// 与 nftables 命令行工具，参数全部由 Config 提供，因此可直接复用于其他代理
// 核心（sing-box、xray 等）的管理工具。以后若有跨项目需求，本目录可整体
// 抽出为独立 Go module。
//
// 回环避免两种方式（与旧脚本 BYPASS_RULE 语义一致）：
//   - ExcludeGID > 0：meta skgid <gid> return（优先）
//   - RoutingMark > 0：meta mark <mark> return（备选）
//   - 两者至少其一，否则拒绝执行。
package intercept

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Config struct {
	TproxyPort   int      // UDP TPROXY 监听端口
	RedirectPort int      // TCP REDIRECT 端口（redir-tproxy 模式）
	Fwmark       int      // 流量打标值
	TableID      int      // 策略路由表 ID
	NftTable     string   // nftables 表名
	ExcludeGID   int      // 回环避免方式一：按 GID 放行
	RoutingMark  int      // 回环避免方式二：按 mark 放行
	BypassCIDRs  []string // 旁路网段（默认保留网段）
}

// 与旧脚本一致的保留网段旁路列表。
var DefaultBypassCIDRs = []string{
	"10.0.0.0/8",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"224.0.0.0/4",
	"240.0.0.0/4",
}

const execTimeout = 15 * time.Second

func (c Config) Validate() error {
	if c.TproxyPort <= 0 {
		return fmt.Errorf("tproxy_port 未设置")
	}
	if c.Fwmark <= 0 || c.TableID <= 0 || c.NftTable == "" {
		return fmt.Errorf("fwmark/table_id/nftables_table 未设置完整")
	}
	if c.ExcludeGID <= 0 && c.RoutingMark <= 0 {
		return fmt.Errorf("exclude_gid 与 routing_mark 必须至少配置其一（回环避免）")
	}
	return nil
}

// bypassRule 生成旁路放行规则，gid 优先。
func (c Config) bypassRule() string {
	if c.ExcludeGID > 0 {
		return fmt.Sprintf("meta skgid %d return", c.ExcludeGID)
	}
	return fmt.Sprintf("meta mark %d return", c.RoutingMark)
}

func (c Config) bypassSet() string {
	cidrs := c.BypassCIDRs
	if len(cidrs) == 0 {
		cidrs = DefaultBypassCIDRs
	}
	return "{ type ipv4_addr; flags interval; elements = { " + strings.Join(cidrs, ", ") + " } }"
}

// ─── 命令执行（无 shell，参数直传） ───────────────────────────

// run 执行一条命令，失败时返回含 stderr 的错误。命令失败即中断（幂等清理
// 场景请用 runIgnore）。
func run(cmd string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", cmd, strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

// runIgnore 执行命令并忽略失败——用于"先删后建"的清理步骤（目标可能不存在）。
func runIgnore(cmd string, args ...string) {
	_ = run(cmd, args...)
}

// ip / nft 快捷封装
func ip(args ...string) error  { return run("ip", args...) }
func nft(args ...string) error { return run("nft", args...) }

func ipIgnore(args ...string)  { runIgnore("ip", args...) }
func nftIgnore(args ...string) { runIgnore("nft", args...) }

// ─── 状态检测 ────────────────────────────────────────────────

// NftTableExists 检查指定 nft 表是否存在（只读检测，供状态页/reconcile 使用）。
func NftTableExists(nftTable string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nft", "list", "tables").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("nft list tables: %s", strings.TrimSpace(string(out)))
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "table ip "+nftTable {
			return true, nil
		}
	}
	return false, nil
}

// RulesApplied 检查模式规则是否就位：nft 表存在 + fwmark 路由规则存在。
// 用于同生共死监控与启动 reconcile 的期望状态对比。
func (c Config) RulesApplied() (bool, error) {
	ok, err := NftTableExists(c.NftTable)
	if err != nil || !ok {
		return ok, err
	}
	return FwmarkRuleExists(c.Fwmark, c.TableID)
}

// FwmarkRuleExists 通过 ip 命令检查 fwmark 规则是否存在。
func FwmarkRuleExists(fwmark, tableID int) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ip", "rule", "show").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("ip rule show: %s", strings.TrimSpace(string(out)))
	}
	// ip rule show 输出形如: "100:	from all fwmark 0x1 lookup 100"
	mark := fmt.Sprintf("fwmark 0x%x", fwmark)
	lookup := fmt.Sprintf("lookup %d", tableID)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, mark) && strings.Contains(line, lookup) {
			return true, nil
		}
	}
	return false, nil
}
