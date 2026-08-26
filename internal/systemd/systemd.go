// Package systemd 封装与 systemd 的 dbus 交互：单元启停、状态查询、
// 状态变化订阅（实时监控的数据源）。
package systemd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
)

const opTimeout = 60 * time.Second

type Client struct {
	conn *dbus.Conn
}

// Normalize 补全单元名后缀："singbox@tun" → "singbox@tun.service"。
// 不带后缀的实例名会在部分 systemd 版本的 StartUnit 校验中被拒绝
// （"Unit name %s is not valid."），systemctl 客户端也是先补后缀再调 dbus。
// 另外 ListUnits 返回的快照 key 恒为完整名，消费方必须用归一化名匹配。
func Normalize(unit string) string {
	if unit == "" {
		return unit
	}
	// 已带后缀（如 singbox@tun.service / foo.socket）则原样返回
	if i := strings.LastIndex(unit, "."); i >= 0 && i > strings.LastIndex(unit, "@") {
		return unit
	}
	return unit + ".service"
}

func New() (*Client, error) {
	// 注意：不能传带超时的 ctx 并随后 cancel——godbus 的 WithContext(ctx)
	// 会在 ctx 取消时关闭连接，这里用 Background，生命周期由 Close() 管理。
	conn, err := dbus.NewSystemConnectionContext(context.Background())
	if err != nil {
		return nil, fmt.Errorf("连接 systemd dbus 失败: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() {
	c.conn.Close()
}

// Start 启动单元并等待任务完成（"done" 或 "failed"）。
func (c *Client) Start(ctx context.Context, unit string) error {
	unit = Normalize(unit)
	ch := make(chan string)
	if _, err := c.conn.StartUnitContext(ctx, unit, "replace", ch); err != nil {
		return fmt.Errorf("启动 %s 失败: %w", unit, err)
	}
	return waitJob(unit, ch)
}

// Stop 停止单元并等待任务完成。
func (c *Client) Stop(ctx context.Context, unit string) error {
	unit = Normalize(unit)
	ch := make(chan string)
	if _, err := c.conn.StopUnitContext(ctx, unit, "replace", ch); err != nil {
		return fmt.Errorf("停止 %s 失败: %w", unit, err)
	}
	return waitJob(unit, ch)
}

func waitJob(unit string, ch chan string) error {
	select {
	case res := <-ch:
		switch res {
		case "done", "skipped":
			return nil
		case "failed":
			return fmt.Errorf("%s 任务失败", unit)
		default:
			return fmt.Errorf("%s 任务异常: %s", unit, res)
		}
	case <-time.After(opTimeout):
		return fmt.Errorf("%s 操作超时(%v)", unit, opTimeout)
	}
}

// ActiveState 返回单元当前 ActiveState；查询失败返回 "unknown"。
func (c *Client) ActiveState(ctx context.Context, unit string) string {
	prop, err := c.conn.GetUnitPropertyContext(ctx, Normalize(unit), "ActiveState")
	if err != nil {
		return "unknown"
	}
	state, ok := prop.Value.Value().(string)
	if !ok {
		return "unknown"
	}
	return state
}

// IsActive 判断单元是否处于活跃状态（含 activating，等价旧脚本 is_service_alive）。
func (c *Client) IsActive(ctx context.Context, unit string) bool {
	switch c.ActiveState(ctx, unit) {
	case "active", "activating", "reloading":
		return true
	}
	return false
}

// SubscribeStates 订阅单元状态变化：每 interval 推送一次「发生变化的单元」
// 快照（map[unit]*UnitStatus，被删除的单元值为 nil）。未变化的单元不在
// 快照中，消费方需自行维护上一次状态做对比。
func (c *Client) SubscribeStates(interval time.Duration) (<-chan map[string]*dbus.UnitStatus, <-chan error) {
	return c.conn.SubscribeUnits(interval)
}
