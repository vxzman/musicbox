// Package lifecycle 是规则生命周期的编排者，守护进程的唯一写入口：
//
//   - StartMode / StopMode：模式互斥切换、启动后延迟套规则、停止后清理
//   - watchUnits：服务状态联动 —— sing-box@ 实例死亡即清规则（同生共死）
//   - watchTun：tun0 消失后清理残留 ip rule / nft 表
//   - reconcile：定期对比期望状态与实际状态，兜底自愈（守护崩溃/手工干预）
//
// 去抖：StartMode 记录「最近编排时间」，延迟窗口内 reconcile 不做规则自愈，
// 避免与延迟套规则打架。
package lifecycle

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"musicbox/internal/config"
	"musicbox/internal/intercept"
	"musicbox/internal/netlink"
	"musicbox/internal/service"
)

type Manager struct {
	cfg *config.ManagerConfig
	sys service.Manager

	mu     sync.Mutex // 串行化模式操作与规则写入
	recent map[string]time.Time
}

func New(cfg *config.ManagerConfig, sys service.Manager) *Manager {
	return &Manager{cfg: cfg, sys: sys, recent: map[string]time.Time{}}
}

func (m *Manager) Mode(name string) *config.Mode {
	return m.cfg.Modes[name]
}

// debounceWindow 内发生的「单元活跃但规则缺失」视为启动编排窗口，不介入。
func (m *Manager) debounceWindow() time.Duration {
	return time.Duration(m.cfg.Daemon.ApplyDelayMs)*time.Millisecond*2 + time.Second
}

func (m *Manager) inDebounce(mode string) bool {
	t, ok := m.recent[mode]
	if !ok {
		return false
	}
	if time.Since(t) > m.debounceWindow() {
		delete(m.recent, mode)
		return false
	}
	return true
}

// ─── 模式操作 ────────────────────────────────────────────────

func (m *Manager) StartMode(ctx context.Context, mode string) error {
	md := m.Mode(mode)
	if md == nil {
		return fmt.Errorf("未知模式: %s", mode)
	}

	// 用户自管模式（无 preset）：配置文件必须预先放置，管理器不生成。
	if md.SelfManaged() {
		path := filepath.Join(m.cfg.ConfigDir(), md.Config)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("模式 %s 的配置文件 %s 不存在（该模式配置由用户自管，请自行放置后重试）", mode, path)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 模式互斥：先停其他活跃模式（含其规则），再启目标。
	for other := range m.cfg.Modes {
		if other == mode {
			continue
		}
		if m.sys.IsActive(ctx, m.cfg.Modes[other].Unit) {
			log.Printf("[lifecycle] 停止互斥模式 %s", other)
			if err := m.stopModeLocked(ctx, other); err != nil {
				return fmt.Errorf("停止互斥模式 %s 失败: %w", other, err)
			}
		}
	}

	log.Printf("[lifecycle] 启动模式 %s (%s)", mode, md.Unit)
	if err := m.sys.Start(ctx, md.Unit); err != nil {
		return err
	}

	// 等单元进入 active（比固定 sleep 更可靠），再延迟套规则。
	if err := m.waitActive(ctx, md.Unit, 20*time.Second); err != nil {
		return err
	}

	if ic, ok := m.interceptConfig(mode); ok {
		m.recent[mode] = time.Now() // 进入去抖窗口
		delay := time.Duration(m.cfg.Daemon.ApplyDelayMs) * time.Millisecond
		time.Sleep(delay)
		log.Printf("[lifecycle] 套用 %s 规则", mode)
		if err := m.applyRules(mode, ic); err != nil {
			return fmt.Errorf("套用 %s 规则失败: %w", mode, err)
		}
	}
	return nil
}

func (m *Manager) StopMode(ctx context.Context, mode string) error {
	md := m.Mode(mode)
	if md == nil {
		return fmt.Errorf("未知模式: %s", mode)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopModeLocked(ctx, mode)
}

// stopModeLocked 停止单元并立即清理规则（watcher 兜底双保险）。
func (m *Manager) stopModeLocked(ctx context.Context, mode string) error {
	md := m.cfg.Modes[mode]
	if err := m.sys.Stop(ctx, md.Unit); err != nil {
		return err
	}
	delete(m.recent, mode)
	if ic, ok := m.interceptConfig(mode); ok {
		m.removeRules(mode, ic)
	}
	return nil
}

// waitActive 轮询单元状态直到 active。
func (m *Manager) waitActive(ctx context.Context, unit string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.sys.IsActive(ctx, unit) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("%s 在 %v 内未进入 active", unit, timeout)
}

// interceptConfig 把 manager.yaml 的 env 映射为 intercept.Config。
func (m *Manager) interceptConfig(mode string) (intercept.Config, bool) {
	md := m.cfg.Modes[mode]
	if md.Env == nil {
		return intercept.Config{}, false
	}
	e := md.Env
	return intercept.Config{
		TproxyPort:   e.TproxyPort,
		RedirectPort: e.RedirectPort,
		Fwmark:       e.Fwmark,
		TableID:      e.TableID,
		NftTable:     e.NftablesTable,
		ExcludeGID:   e.ExcludeGID,
		RoutingMark:  e.RoutingMark,
	}, true
}

func (m *Manager) applyRules(mode string, ic intercept.Config) error {
	switch mode {
	case "tproxy":
		return intercept.ApplyTproxy(ic)
	case "redir-tproxy":
		return intercept.ApplyRedirTproxy(ic)
	}
	return nil
}

func (m *Manager) removeRules(mode string, ic intercept.Config) {
	switch mode {
	case "tproxy":
		intercept.RemoveTproxy(ic)
	case "redir-tproxy":
		intercept.RemoveRedirTproxy(ic)
	case "tun":
		m.cleanupTun()
	}
}

// cleanupTun 清理 tun 模式残留（幂等）。
func (m *Manager) cleanupTun() {
	md := m.cfg.Modes["tun"]
	if md == nil || md.Routing == nil {
		return
	}
	tables := []string{}
	if md.Cleanup != nil {
		tables = md.Cleanup.NftTables
	}
	if err := intercept.CleanupTun(md.Routing.TableIndex, tables); err != nil {
		log.Printf("[lifecycle] tun 清理失败: %v", err)
	}
}

// tunDevice 从 tun preset 解析 tun 接口名，默认 tun0。
func (m *Manager) tunDevice() string {
	md := m.cfg.Modes["tun"]
	if md == nil || md.Preset == "" {
		return "tun0"
	}
	var listeners []map[string]interface{}
	if err := yaml.Unmarshal([]byte(md.Preset), &listeners); err != nil {
		return "tun0"
	}
	for _, l := range listeners {
		if l["type"] == "tun" {
			if dev, ok := l["device"].(string); ok && dev != "" {
				return dev
			}
		}
	}
	return "tun0"
}

// ─── 守护循环：watcher + reconcile ──────────────────────────

// Run 启动 unit 状态监听（同生共死）、tun0 事件监听与定期 reconcile，
// 直到 ctx 取消。
func (m *Manager) Run(ctx context.Context) error {
	m.reconcileOnce(ctx) // 启动全量对账：兜住守护崩溃/重启的残局

	go m.watchTun(ctx)

	// 订阅间隔取对账间隔的一半（下限 1s），保证状态变化及时感知；
	// 同生共死以 reconcile 周期为最终兜底。
	interval := 5 * time.Second
	if d, err := time.ParseDuration(m.cfg.Daemon.ReconcileInterval); err == nil && d > 0 {
		interval = d
	}
	subInterval := interval / 2
	if subInterval < time.Second {
		subInterval = time.Second
	}

	updates, errCh := m.sys.SubscribeStates(subInterval)

	// 快照 key 恒为完整单元名（带 .service 后缀），统一用归一化名匹配。
	prevActive := map[string]bool{}
	unitNames := map[string]string{} // mode → normalized unit
	for _, md := range m.cfg.Modes {
		full := service.Normalize(md.Unit)
		unitNames[md.Unit] = full
		prevActive[full] = m.sys.IsActive(ctx, md.Unit)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			log.Printf("[lifecycle] 服务状态订阅错误: %v", err)
		case snapshot := <-updates:
			for mode, md := range m.cfg.Modes {
				full := unitNames[md.Unit]
				p, present := snapshot[full]
				if !present {
					continue // 未变化的单元不在快照中
				}
				active := false
				if p != nil {
					switch p.ActiveState {
					case "active", "activating", "reloading":
						active = true
					}
				}
				if prevActive[full] && !active {
					// sing-box@ 实例死亡 → 清规则（同生共死）
					log.Printf("[lifecycle] %s 已停止，清理规则", md.Unit)
					m.onUnitDown(mode)
				}
				prevActive[full] = active
			}
		case <-ticker.C:
			m.reconcileOnce(ctx)
		}
	}
}

// onUnitDown 在 sing-box@ 实例停止后清理对应模式规则。
func (m *Manager) onUnitDown(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ic, ok := m.interceptConfig(mode); ok {
		m.removeRules(mode, ic)
	}
}

// watchTun 监听 tun0 消失事件，触发清理。
func (m *Manager) watchTun(ctx context.Context) {
	done := ctx.Done()
	events, err := netlink.WatchLinkDown(m.tunDevice(), done)
	if err != nil {
		log.Printf("[lifecycle] tun0 事件订阅失败: %v", err)
		return
	}
	for ev := range events {
		log.Printf("[lifecycle] tun 接口 %s down（deleted=%v），清理残留", ev.Name, ev.Deleted)
		m.mu.Lock()
		m.cleanupTun()
		m.mu.Unlock()
	}
}

// reconcileOnce 对比期望状态与实际状态并自愈。
func (m *Manager) reconcileOnce(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for mode, md := range m.cfg.Modes {
		unitUp := m.sys.IsActive(ctx, md.Unit)

		switch mode {
		case "tproxy", "redir-tproxy":
			ic, ok := m.interceptConfig(mode)
			if !ok {
				continue
			}
			applied, err := ic.RulesApplied()
			if err != nil {
				log.Printf("[lifecycle] %s 规则检测失败: %v", mode, err)
				continue
			}
			switch {
			case unitUp && !applied && !m.inDebounce(mode):
				log.Printf("[lifecycle] reconcile: %s 单元活跃但规则缺失，自愈套用", mode)
				if err := m.applyRules(mode, ic); err != nil {
					log.Printf("[lifecycle] %s 自愈套用失败: %v", mode, err)
				}
			case !unitUp && applied:
				log.Printf("[lifecycle] reconcile: %s 单元已停但规则残留，清理", mode)
				m.removeRules(mode, ic)
			}

		case "tun":
			md := m.cfg.Modes["tun"]
			if md == nil || md.Routing == nil {
				continue
			}
			leftover, err := intercept.TunRulesLeftover(md.Routing.TableIndex)
			if err != nil {
				continue
			}
			if leftover && !unitUp {
				log.Printf("[lifecycle] reconcile: tun 残留规则，清理")
				m.cleanupTun()
			}
		}
	}
}

// ─── 状态汇总 ────────────────────────────────────────────────

type ModeStatus struct {
	Label     string `json:"label"`
	Unit      string `json:"unit"`
	UnitState string `json:"unit_state"` // active|inactive|failed|unknown
	Rules     string `json:"rules"`      // present|missing|clean|leftover|na
	Active    bool   `json:"active"`
}

type Status struct {
	Modes      map[string]ModeStatus `json:"modes"`
	ActiveMode string                `json:"active_mode"`
}

func (m *Manager) Status(ctx context.Context) *Status {
	st := &Status{Modes: map[string]ModeStatus{}}
	for name, md := range m.cfg.Modes {
		state := m.sys.ActiveState(ctx, md.Unit)
		ms := ModeStatus{
			Label:     md.Label,
			Unit:      md.Unit,
			UnitState: state,
			Rules:     "na",
			Active:    state == "active" || state == "activating",
		}
		switch name {
		case "tproxy", "redir-tproxy":
			applied := false
			if ic, ok := m.interceptConfig(name); ok {
				if a, err := ic.RulesApplied(); err == nil {
					applied = a
				}
			}
			switch {
			case applied && ms.Active:
				ms.Rules = "present" // 运行中，规则已就位
			case applied:
				ms.Rules = "leftover" // 已停止但规则残留
			case ms.Active:
				ms.Rules = "missing" // 运行中但规则缺失
			default:
				ms.Rules = "clean" // 已停止且无残留
			}
		case "tun":
			// tun 运行时 sing-box 自己会向路由表加规则（auto-route 的正常行为），
			// 只有「单元已停但规则仍在」才算残留。
			ms.Rules = "clean"
			if md.Routing != nil {
				leftover, err := intercept.TunRulesLeftover(md.Routing.TableIndex)
				if err == nil {
					switch {
					case leftover && ms.Active:
						ms.Rules = "present" // 运行中，策略规则已建立
					case leftover:
						ms.Rules = "leftover" // 已停止但规则残留
					case ms.Active:
						ms.Rules = "missing" // 运行中但规则尚未建立
					}
				}
			}
		}
		st.Modes[name] = ms
	}
	// 非容器：以 systemd 上真实运行的 sing-box@<mode> 为准覆盖首页活跃模式。
	for _, inst := range m.sys.ActiveInstances(ctx, "sing-box") {
		ms, ok := st.Modes[inst]
		if !ok {
			continue
		}
		wasActive := ms.Active
		ms.Active = true
		if ms.UnitState != "active" && ms.UnitState != "activating" && ms.UnitState != "reloading" {
			ms.UnitState = "active"
		}
		if !wasActive {
			switch ms.Rules {
			case "clean":
				ms.Rules = "missing"
			case "leftover":
				ms.Rules = "present"
			}
		}
		st.Modes[inst] = ms
	}
	for _, name := range config.BuiltinModes {
		if ms, ok := st.Modes[name]; ok && ms.Active {
			st.ActiveMode = name
			break
		}
	}
	return st
}
