package intercept

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ─── TUN 模式清理 ────────────────────────────────────────────
// singbox 的 auto-route/auto-redirect 自建 tun0 与策略路由规则。tun0 消失后
// 可能残留 ip rule（指向 iproute2-table-index 表）与 nft 表，需要清理。
// 与旧 tun_cleanup 的差异：规则数字从 manager.yaml 的 routing.table_index
// 派生，不再硬编码 8999/9000/9001/9002/9010。

// CleanupTun 删除指向 tableIndex 的全部 ip rule、flush 路由表、删除 nft 表
// 列表（表名含 family，如 "inet singbox"）。全部幂等。
func CleanupTun(tableIndex int, nftTables []string) error {
	table := strconv.Itoa(tableIndex)

	// 删除指向该表的全部 ip rule：解析 `ip rule show` 输出提取 priority。
	// singbox 会按 rule_index 连续加多条规则，按表名匹配比固定 pref 列表更稳。
	for i := 0; i < 64; i++ {
		prefs, err := rulePrioritiesForTable(table)
		if err != nil {
			return err
		}
		if len(prefs) == 0 {
			break
		}
		for _, pref := range prefs {
			ipIgnore("rule", "del", "pref", pref)
		}
	}

	ipIgnore("route", "flush", "table", table)

	for _, t := range nftTables {
		parts := strings.Fields(t)
		if len(parts) == 2 { // "inet singbox" / "ip singbox_tproxy4"
			nftIgnore(append([]string{"delete", "table"}, parts...)...)
		}
	}
	return nil
}

// rulePrioritiesForTable 返回 `ip rule show` 中 lookup <table> 规则的 priority 列表。
func rulePrioritiesForTable(table string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ip", "rule", "show").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ip rule show: %s", strings.TrimSpace(string(out)))
	}
	lookup := "lookup " + table
	var prefs []string
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, lookup) {
			continue
		}
		// 行格式: "9000:	from all lookup 2022"
		idx := strings.Index(line, ":")
		if idx > 0 {
			prefs = append(prefs, strings.TrimSpace(line[:idx]))
		}
	}
	return prefs, nil
}

// TunRulesLeftover 检测指定路由表是否残留规则（只读检测，供状态页/reconcile）。
func TunRulesLeftover(tableIndex int) (bool, error) {
	prefs, err := rulePrioritiesForTable(strconv.Itoa(tableIndex))
	if err != nil {
		return false, err
	}
	return len(prefs) > 0, nil
}
