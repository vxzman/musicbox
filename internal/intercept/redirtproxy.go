package intercept

import (
	"fmt"
)

// ─── REDIR-TPROXY 混合模式 ───────────────────────────────────
// TCP REDIRECT + UDP TPROXY。语义照搬旧 redir-tproxy.sh。

func (c Config) validateRedir() error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("redir-tproxy 配置无效: %w", err)
	}
	if c.RedirectPort <= 0 {
		return fmt.Errorf("redir-tproxy 配置无效: redirect_port 未设置")
	}
	return nil
}

// ApplyRedirTproxy 落地混合模式全套规则。
func ApplyRedirTproxy(c Config) error {
	if err := c.validateRedir(); err != nil {
		return err
	}
	removeRedirTproxy(c)
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

	chains := [][]string{
		{"add", "chain", "ip", c.NftTable, "RDR_LAN", "{ type nat hook prerouting priority -100; policy accept; }"},
		{"add", "chain", "ip", c.NftTable, "RDR_SELF", "{ type nat hook output priority -100; policy accept; }"},
		{"add", "chain", "ip", c.NftTable, "TPROXY_LAN", "{ type filter hook prerouting priority -150; policy accept; }"},
		{"add", "chain", "ip", c.NftTable, "TPROXY_SELF", "{ type route hook output priority -150; policy accept; }"},
	}
	for _, ch := range chains {
		if err := nft(ch...); err != nil {
			return err
		}
	}

	// ── TCP REDIRECT（LAN 入站 + 本机发出） ──
	rdr := [][]string{
		{"add", "rule", "ip", c.NftTable, "RDR_LAN", "ip protocol != tcp return"},
		{"add", "rule", "ip", c.NftTable, "RDR_LAN", c.bypassRule()},
		{"add", "rule", "ip", c.NftTable, "RDR_LAN", "ip daddr @BYPASS_LIST udp dport != 53 return"},
		{"add", "rule", "ip", c.NftTable, "RDR_LAN", "ip daddr @BYPASS_LIST tcp dport != 53 return"},
		{"add", "rule", "ip", c.NftTable, "RDR_LAN", fmt.Sprintf("ip protocol tcp redirect to :%d", c.RedirectPort)},
		{"add", "rule", "ip", c.NftTable, "RDR_SELF", "ip protocol != tcp return"},
		{"add", "rule", "ip", c.NftTable, "RDR_SELF", c.bypassRule()},
		{"add", "rule", "ip", c.NftTable, "RDR_SELF", "ip daddr @BYPASS_LIST udp dport != 53 return"},
		{"add", "rule", "ip", c.NftTable, "RDR_SELF", "ip daddr @BYPASS_LIST tcp dport != 53 return"},
		{"add", "rule", "ip", c.NftTable, "RDR_SELF", fmt.Sprintf("ip protocol tcp redirect to :%d", c.RedirectPort)},
	}
	for _, r := range rdr {
		if err := nft(r...); err != nil {
			return err
		}
	}

	// ── UDP TPROXY（LAN 入站 + 本机发出） ──
	tp := [][]string{
		{"add", "rule", "ip", c.NftTable, "TPROXY_LAN", "ip protocol != udp return"},
		{"add", "rule", "ip", c.NftTable, "TPROXY_LAN", c.bypassRule()},
		{"add", "rule", "ip", c.NftTable, "TPROXY_LAN", "ip daddr @BYPASS_LIST udp dport != 53 return"},
		{"add", "rule", "ip", c.NftTable, "TPROXY_LAN",
			fmt.Sprintf("ip protocol udp tproxy to 127.0.0.1:%d meta mark set %d", c.TproxyPort, c.Fwmark)},
		{"add", "rule", "ip", c.NftTable, "TPROXY_SELF", "ip protocol != udp return"},
		{"add", "rule", "ip", c.NftTable, "TPROXY_SELF", c.bypassRule()},
		{"add", "rule", "ip", c.NftTable, "TPROXY_SELF", "ip daddr @BYPASS_LIST udp dport != 53 return"},
		{"add", "rule", "ip", c.NftTable, "TPROXY_SELF", fmt.Sprintf("ip protocol udp meta mark set %d", c.Fwmark)},
	}
	for _, r := range tp {
		if err := nft(r...); err != nil {
			return err
		}
	}
	return nil
}

// RemoveRedirTproxy 清理混合模式全套规则（幂等）。
func RemoveRedirTproxy(c Config) {
	removeRedirTproxy(c)
}

func removeRedirTproxy(c Config) {
	nftIgnore("delete", "table", "ip", c.NftTable)
	removeRoutes(c)
}
