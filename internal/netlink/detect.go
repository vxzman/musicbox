// Package netlink 通过 vishvananda/netlink 做只读状态检测：
// ip rule / 路由表枚举（对比期望状态）+ 网卡事件订阅（tun0 消失清理）。
// 本包不写任何规则——规则增删统一走 internal/intercept 的 ip/nft 命令。
package netlink

import (
	"syscall"

	"github.com/vishvananda/netlink"
)

// FwmarkRuleExists 检查 fwmark 路由规则是否存在（netlink 只读查询）。
func FwmarkRuleExists(fwmark uint32, table int) (bool, error) {
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		return false, err
	}
	for _, r := range rules {
		if r.Mark == fwmark && r.Table == table {
			return true, nil
		}
	}
	return false, nil
}

// RulesForTable 返回指向指定路由表的全部规则优先级（tun 残留检测用）。
func RulesForTable(table int) ([]int, error) {
	rules, err := netlink.RuleList(netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	var prios []int
	for _, r := range rules {
		if r.Table == table {
			prios = append(prios, r.Priority)
		}
	}
	return prios, nil
}

// RoutesForTable 返回指定路由表内的路由（tun 残留检测用）。
func RoutesForTable(table int) ([]netlink.Route, error) {
	return netlink.RouteListFiltered(netlink.FAMILY_V4,
		&netlink.Route{Table: table}, netlink.RT_FILTER_TABLE)
}

// LinkExists 判断接口是否存在。
func LinkExists(name string) (bool, error) {
	_, err := netlink.LinkByName(name)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// LinkDown 表示目标接口 down 或被删除的事件。
type LinkDown struct {
	Name    string
	Deleted bool // 接口被删除（RTM_DELLINK）
}

// WatchLinkDown 订阅网卡事件，仅转发目标接口的 down/删除事件。
// done 关闭后订阅退出。
func WatchLinkDown(iface string, done <-chan struct{}) (<-chan LinkDown, error) {
	updates := make(chan netlink.LinkUpdate)
	if err := netlink.LinkSubscribe(updates, done); err != nil {
		return nil, err
	}
	out := make(chan LinkDown, 8)
	go func() {
		defer close(out)
		for {
			select {
			case <-done:
				return
			case u, ok := <-updates:
				if !ok {
					return
				}
				if u.Link.Attrs().Name != iface {
					continue
				}
				ev := LinkDown{Name: iface}
				switch {
				case u.Header.Type == syscall.RTM_DELLINK:
					ev.Deleted = true
					out <- ev
				case u.Link.Attrs().OperState == netlink.OperDown:
					out <- ev
				}
			}
		}
	}()
	return out, nil
}
