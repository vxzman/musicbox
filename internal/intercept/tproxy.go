package intercept

import (
	"fmt"
	"strconv"
)

// ─── 路由（tproxy 与 redir-tproxy 共用） ─────────────────────

func setupRoutes(c Config) error {
	// 先删后建，保证幂等
	ipIgnore("rule", "del", "fwmark", strconv.Itoa(c.Fwmark), "table", strconv.Itoa(c.TableID))
	if err := ip("rule", "add", "fwmark", strconv.Itoa(c.Fwmark), "table", strconv.Itoa(c.TableID)); err != nil {
		return err
	}
	ipIgnore("route", "flush", "table", strconv.Itoa(c.TableID))
	return ip("route", "add", "local", "0.0.0.0/0", "dev", "lo", "table", strconv.Itoa(c.TableID))
}

func removeRoutes(c Config) {
	ipIgnore("rule", "del", "fwmark", strconv.Itoa(c.Fwmark), "table", strconv.Itoa(c.TableID))
	ipIgnore("route", "flush", "table", strconv.Itoa(c.TableID))
}

// ─── TPROXY 模式 ─────────────────────────────────────────────

// Apply 落地 TPROXY 全套规则：路由 + nft 表/集合/链/规则。
// 语义照搬旧 tproxy.sh：TCP+UDP 双透明代理，旁路保留网段与 DNS 放行。
func ApplyTproxy(c Config) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("tproxy 配置无效: %w", err)
	}
	removeTproxy(c)
	if err := setupRoutes(c); err != nil {
		return err
	}

	nftIgnore("delete", "table", "ip", c.NftTable)
	if err := nft("add", "table", "ip", c.NftTable); err != nil {
		return err
	}
	if err := nft("add", "set", "ip", c.NftTable, "BYPASS_LIST", c.bypassSet()); err != nil {
		return err
	}

	// ── TPROXY_FORWARD：局域网入站（filter prerouting） ──
	if err := nft("add", "chain", "ip", c.NftTable, "TPROXY_FORWARD",
		"{ type filter hook prerouting priority -150; policy accept; }"); err != nil {
		return err
	}
	rules := [][]string{
		{"add", "rule", "ip", c.NftTable, "TPROXY_FORWARD", c.bypassRule()},
		{"add", "rule", "ip", c.NftTable, "TPROXY_FORWARD", "ip daddr @BYPASS_LIST udp dport != 53 return"},
		{"add", "rule", "ip", c.NftTable, "TPROXY_FORWARD", "ip daddr @BYPASS_LIST tcp dport != 53 return"},
		{"add", "rule", "ip", c.NftTable, "TPROXY_FORWARD",
			fmt.Sprintf("ip protocol { tcp, udp } meta mark set %d tproxy to 127.0.0.1:%d", c.Fwmark, c.TproxyPort)},
	}
	for _, r := range rules {
		if err := nft(r...); err != nil {
			return err
		}
	}

	// ── TPROXY_LOCAL：本机发出（route output） ──
	if err := nft("add", "chain", "ip", c.NftTable, "TPROXY_LOCAL",
		"{ type route hook output priority -150; policy accept; }"); err != nil {
		return err
	}
	localRules := [][]string{
		{"add", "rule", "ip", c.NftTable, "TPROXY_LOCAL", c.bypassRule()},
		{"add", "rule", "ip", c.NftTable, "TPROXY_LOCAL", "ip daddr @BYPASS_LIST udp dport != 53 return"},
		{"add", "rule", "ip", c.NftTable, "TPROXY_LOCAL", "ip daddr @BYPASS_LIST tcp dport != 53 return"},
		{"add", "rule", "ip", c.NftTable, "TPROXY_LOCAL",
			fmt.Sprintf("ip protocol { tcp, udp } meta mark set %d", c.Fwmark)},
	}
	for _, r := range localRules {
		if err := nft(r...); err != nil {
			return err
		}
	}
	return nil
}

// Remove 清理 TPROXY 全套规则（幂等）。
func RemoveTproxy(c Config) {
	removeTproxy(c)
}

func removeTproxy(c Config) {
	nftIgnore("delete", "table", "ip", c.NftTable)
	removeRoutes(c)
}
