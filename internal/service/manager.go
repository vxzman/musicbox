// Package service 定义 lifecycle 使用的服务管理契约。
// 后端可以是 systemd，也可以是容器内的进程监督器，对上层隐藏实现。
package service

import (
	"context"
	"strings"
	"time"
)

type UnitStatus struct {
	Name        string
	Description string
	LoadState   string
	ActiveState string
	SubState    string
}

type Manager interface {
	Start(ctx context.Context, unit string) error
	Stop(ctx context.Context, unit string) error
	ActiveState(ctx context.Context, unit string) string
	IsActive(ctx context.Context, unit string) bool
	SubscribeStates(interval time.Duration) (<-chan map[string]*UnitStatus, <-chan error)
	// ActiveInstances 列出正在运行的 template@instance（如 sing-box@tun → "tun"）。
	// 容器后端返回 nil：只认本进程拉起的子进程。
	ActiveInstances(ctx context.Context, template string) []string
	Close()
}

// Normalize 补全单元名后缀："sing-box@tun" → "sing-box@tun.service"。
func Normalize(unit string) string {
	if unit == "" {
		return unit
	}
	if i := strings.LastIndex(unit, "."); i >= 0 && i > strings.LastIndex(unit, "@") {
		return unit
	}
	return unit + ".service"
}
