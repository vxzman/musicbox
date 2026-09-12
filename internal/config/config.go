package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Path 返回 manager.yaml 的位置：MUSICBOX_CONFIG 优先（dev/测试用），
// 也兼容旧环境变量 SINGBOX_MANAGER_CONFIG，
// 默认 /opt/musicbox/manager.yaml。
func Path() string {
	if v := os.Getenv("MUSICBOX_CONFIG"); v != "" {
		return v
	}
	if v := os.Getenv("SINGBOX_MANAGER_CONFIG"); v != "" {
		return v
	}
	return DefaultConfigPath
}

// Load 读取 manager.yaml；文件不存在时先尝试从老布局迁移（/etc/sing-box 下的
// .conf 或 /opt/singbox-manager/manager.yaml），迁移不到就用内置默认值，并把结果落盘——保证首启即可用。
func Load() (*ManagerConfig, error) {
	path := Path()
	data, err := os.ReadFile(path)
	if err == nil {
		var cfg ManagerConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %w", path, err)
		}
		fillDefaults(&cfg)
		changed := pruneRetiredModes(&cfg)
		changed = canonicalizeDirs(&cfg) || changed
		if changed {
			_ = Save(&cfg)
		}
		return &cfg, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	// 兼容迁移：如果 /opt/musicbox/manager.yaml 不存在，但 /opt/singbox-manager/manager.yaml 存在，
	// 则自动读取旧配置并落盘至新路径
	if path == DefaultConfigPath {
		if oldData, oldErr := os.ReadFile(LegacyConfigPath); oldErr == nil {
			var oldCfg ManagerConfig
			if yaml.Unmarshal(oldData, &oldCfg) == nil {
				fillDefaults(&oldCfg)
				pruneRetiredModes(&oldCfg)
				canonicalizeDirs(&oldCfg)
				_ = Save(&oldCfg)
				return &oldCfg, nil
			}
		}
	}

	cfg := Default()
	applyLegacyMigration(cfg)
	fillDefaults(cfg)
	pruneRetiredModes(cfg)
	canonicalizeDirs(cfg)
	if err := Save(cfg); err != nil {
		return nil, fmt.Errorf("生成默认配置 %s 失败: %w", path, err)
	}
	return cfg, nil
}

// Save 原子写入：临时文件 + rename，避免守护进程与 CLI 并发读到半截文件。
// manager.yaml 是机器管理文件，保存时会重排格式、丢弃手写注释。
func Save(cfg *ManagerConfig) error {
	if err := Validate(cfg); err != nil {
		return err
	}
	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// fillDefaults 补齐缺失字段，保证老版本/手改文件升级后结构完整。
func fillDefaults(cfg *ManagerConfig) {
	def := Default()
	if cfg.Dirs.ConfigDir == "" {
		cfg.Dirs.ConfigDir = def.Dirs.ConfigDir
	}
	if cfg.Dirs.DataDir == "" {
		cfg.Dirs.DataDir = def.Dirs.DataDir
	}
	if cfg.Daemon.WebAddr == "" {
		cfg.Daemon.WebAddr = def.Daemon.WebAddr
	}
	// 迁移旧默认值：":8082" 是双栈通配（含 IPv6），面板无鉴权不应默认暴露
	// IPv6——仅当配置仍是旧默认值时替换为 IPv4-only，用户自定义值不动。
	if cfg.Daemon.WebAddr == ":8082" {
		cfg.Daemon.WebAddr = def.Daemon.WebAddr
	}
	if cfg.Daemon.CliSocket == "" || cfg.Daemon.CliSocket == "/run/singbox-manager/singbox-manager.sock" {
		cfg.Daemon.CliSocket = def.Daemon.CliSocket
	}
	if cfg.Daemon.ApplyDelayMs == 0 {
		cfg.Daemon.ApplyDelayMs = def.Daemon.ApplyDelayMs
	}
	if cfg.Daemon.ReconcileInterval == "" {
		cfg.Daemon.ReconcileInterval = def.Daemon.ReconcileInterval
	}
	for name, m := range cfg.Modes {
		if m == nil {
			cfg.Modes[name] = def.Modes[name]
			continue
		}
		if m.Label == "" {
			m.Label = name
		}
		if m.Unit == "" {
			m.Unit = "sing-box@" + name
		}
		if m.Config == "" {
			m.Config = "config_" + name + ".json"
		}
	}
	// 升级补齐：老版本 manager.yaml 缺少的内置模式自动补入。
	for name, dm := range def.Modes {
		if _, ok := cfg.Modes[name]; !ok {
			cfg.Modes[name] = dm
		}
	}
}

func pruneRetiredModes(cfg *ManagerConfig) bool {
	changed := false
	for _, name := range retiredModes {
		if _, ok := cfg.Modes[name]; ok {
			delete(cfg.Modes, name)
			changed = true
		}
	}
	return changed
}

// canonicalizeDirs 把生产环境的配置目录钉在 /etc/sing-box。
// 相对路径 etc-sing-box 只留给 yaml 旁边确实存在该目录的 dev fixture；
// 落到 /opt/musicbox/manager.yaml 时若仍写相对路径，会错误读成
// /opt/musicbox/etc-sing-box。
func canonicalizeDirs(cfg *ManagerConfig) bool {
	changed := false
	switch cfg.Dirs.ConfigDir {
	case "etc-sing-box", "etc-singbox":
		local := filepath.Join(filepath.Dir(Path()), cfg.Dirs.ConfigDir)
		if _, err := os.Stat(local); err != nil {
			cfg.Dirs.ConfigDir = "/etc/sing-box"
			changed = true
		}
	case "/etc/singbox":
		cfg.Dirs.ConfigDir = "/etc/sing-box"
		changed = true
	}
	if cfg.Dirs.DataDir == "/var/lib/singbox" {
		cfg.Dirs.DataDir = "/var/lib/sing-box"
		changed = true
	}
	return changed
}

// Validate 检查结构完整性；tproxy/redir-tproxy 的回环避免必须 gid/mark 至少其一。
func Validate(cfg *ManagerConfig) error {
	if len(cfg.Modes) == 0 {
		return fmt.Errorf("modes 不能为空")
	}
	for name, m := range cfg.Modes {
		if m == nil {
			return fmt.Errorf("模式 %s 定义为空", name)
		}
		if m.Unit == "" || m.Config == "" {
			return fmt.Errorf("模式 %s 缺少 unit/config", name)
		}
		switch name {
		case "tproxy", "redir-tproxy":
			env := m.Env
			if env == nil {
				return fmt.Errorf("模式 %s 缺少 env", name)
			}
			if env.TproxyPort <= 0 || env.Fwmark <= 0 || env.TableID <= 0 || env.NftablesTable == "" {
				return fmt.Errorf("模式 %s env 不完整", name)
			}
			if name == "redir-tproxy" && env.RedirectPort <= 0 {
				return fmt.Errorf("模式 %s 缺少 redirect_port", name)
			}
			if env.ExcludeGID <= 0 && env.RoutingMark <= 0 {
				return fmt.Errorf("模式 %s 必须配置 exclude_gid 或 routing_mark 之一（回环避免）", name)
			}
		case "tun":
			if m.Routing == nil || m.Routing.TableIndex <= 0 {
				return fmt.Errorf("模式 tun 缺少 routing.table_index")
			}
		}
		if m.Preset != "" {
			if err := ValidatePreset(m.Preset); err != nil {
				return fmt.Errorf("模式 %s 预定义配置无效: %w", name, err)
			}
		}
		if m.Endpoints != "" {
			if err := ValidateEndpoints(m.Endpoints); err != nil {
				return fmt.Errorf("模式 %s 端点模块无效: %w", name, err)
			}
		}
	}
	return nil
}

// ConfigDir / DataDir 返回解析后的目录；相对路径以 manager.yaml 所在目录为基准
// （dev fixture 因此可写相对路径、可整体搬移）。
func (c *ManagerConfig) ConfigDir() string {
	return resolveAgainst(c.Dirs.ConfigDir, Path())
}

func (c *ManagerConfig) DataDir() string {
	return resolveAgainst(c.Dirs.DataDir, Path())
}

func resolveAgainst(dir, cfgPath string) string {
	if dir == "" || filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(filepath.Dir(cfgPath), dir)
}

// ─── 老布局迁移 ─────────────────────────────────────────────
// 从旧布局 /etc/sing-box/ 的 .conf（脚本读）与无后缀文件（旧面板读）
// 迁移 env 值；.conf 优先。其余保持内置默认。

func applyLegacyMigration(cfg *ManagerConfig) {
	tproxyEnv := parseLegacyConf("/etc/sing-box/sing-box_tproxy.conf", "/etc/sing-box/sing-box_tproxy")
	mixEnv := parseLegacyConf("/etc/sing-box/sing-box_redir-tproxy.conf", "/etc/sing-box/sing-box_redir-tproxy")

	if e := tproxyEnv; e != nil {
		if m := cfg.Modes["tproxy"]; m != nil && m.Env != nil {
			m.Env.TproxyPort = e.TproxyPort
			m.Env.ExcludeGID = e.ExcludeGID
			m.Env.RoutingMark = e.RoutingMark
			m.Env.Fwmark = e.Fwmark
			m.Env.TableID = e.TableID
			m.Env.NftablesTable = e.NftablesTable
		}
	}
	if e := mixEnv; e != nil {
		if m := cfg.Modes["redir-tproxy"]; m != nil && m.Env != nil {
			m.Env.TproxyPort = e.TproxyPort
			m.Env.RedirectPort = e.RedirectPort
			m.Env.ExcludeGID = e.ExcludeGID
			m.Env.RoutingMark = e.RoutingMark
			m.Env.Fwmark = e.Fwmark
			m.Env.TableID = e.TableID
			m.Env.NftablesTable = e.NftablesTable
		}
	}
}

// parseLegacyConf 解析 KEY=VALUE 格式的旧 env 文件；全部缺失返回 nil。
func parseLegacyConf(paths ...string) *Env {
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		env := &Env{}
		found := false
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			kv := strings.SplitN(line, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key, val := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])
			switch key {
			case "TPROXY_PORT":
				env.TproxyPort, _ = strconv.Atoi(val)
			case "REDIRECT_PORT":
				env.RedirectPort, _ = strconv.Atoi(val)
			case "EXCLUDE_GID":
				env.ExcludeGID, _ = strconv.Atoi(val)
			case "ROUTING_MARK":
				env.RoutingMark, _ = strconv.Atoi(val)
			case "FWMARK":
				env.Fwmark, _ = strconv.Atoi(val)
			case "TABLE_ID":
				env.TableID, _ = strconv.Atoi(val)
			case "NFTABLES_TABLE":
				env.NftablesTable = val
			}
			found = true
		}
		if found {
			return env
		}
	}
	return nil
}
