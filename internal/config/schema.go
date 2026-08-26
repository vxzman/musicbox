package config

// ManagerConfig 是 /opt/singbox-manager/manager.yaml 的类型化视图：
// 所有 ip rule 数字、env 变量、模式定义与预定义入站配置的单一事实源。
// 命名约定：二进制与用户/组保留官方 sing-box，其余统一无连字符 singbox。
type ManagerConfig struct {
	Dirs   Dirs             `yaml:"dirs" json:"dirs"`
	Daemon DaemonSettings   `yaml:"daemon" json:"daemon"`
	Modes  map[string]*Mode `yaml:"modes" json:"modes"`
}

type Dirs struct {
	ConfigDir string `yaml:"config_dir" json:"config_dir"`
	DataDir   string `yaml:"data_dir" json:"data_dir"`
}

type DaemonSettings struct {
	WebAddr           string `yaml:"web_addr" json:"web_addr"`
	CliSocket         string `yaml:"cli_socket" json:"cli_socket"`
	ApplyDelayMs      int    `yaml:"apply_delay_ms" json:"apply_delay_ms"`
	ReconcileInterval string `yaml:"reconcile_interval" json:"reconcile_interval"`
}

type Mode struct {
	Label   string   `yaml:"label" json:"label"`
	Unit    string   `yaml:"unit" json:"unit"`
	Config  string   `yaml:"config" json:"config"`
	Routing *Routing `yaml:"routing,omitempty" json:"routing,omitempty"`
	Cleanup *Cleanup `yaml:"cleanup,omitempty" json:"cleanup,omitempty"`
	Env     *Env     `yaml:"env,omitempty" json:"env,omitempty"`
	Preset  string   `yaml:"preset,omitempty" json:"preset,omitempty"`
}

// Routing 承载 tun 模式的策略路由索引：清理逻辑据此派生，不再硬编码数字。
// sing-box 的 tun auto_route 与 mihomo 使用相同的 iproute2 约定（rule 9000 / table 2022）。
type Routing struct {
	RuleIndex  int `yaml:"rule_index" json:"rule_index"`
	TableIndex int `yaml:"table_index" json:"table_index"`
}

// Cleanup 列出 tun 接口消失后需要清理的 nft 表。
type Cleanup struct {
	NftTables []string `yaml:"nft_tables" json:"nft_tables"`
}

// Env 承载透明代理模式（tproxy / redir-tproxy）的网络参数。
// 回环避免：ExcludeGID（meta skgid）优先，RoutingMark（meta mark）备选，至少其一。
type Env struct {
	TproxyPort    int    `yaml:"tproxy_port" json:"tproxy_port"`
	RedirectPort  int    `yaml:"redirect_port,omitempty" json:"redirect_port,omitempty"`
	ExcludeGID    int    `yaml:"exclude_gid,omitempty" json:"exclude_gid,omitempty"`
	RoutingMark   int    `yaml:"routing_mark,omitempty" json:"routing_mark,omitempty"`
	Fwmark        int    `yaml:"fwmark" json:"fwmark"`
	TableID       int    `yaml:"table_id" json:"table_id"`
	NftablesTable string `yaml:"nftables_table" json:"nftables_table"`
}

// ─── 默认配置 ────────────────────────────────────────────────

const (
	DefaultConfigPath = "/opt/singbox-manager/manager.yaml"

	// 与旧 sing-box 面板/脚本保持一致的默认值，迁移时被旧 .conf 覆盖。
	defaultTproxyPort   = 22026
	defaultRedirectPort = 22025
	defaultExcludeGID   = 988
	defaultRoutingMark  = 6666
	defaultFwmark       = 1
	defaultTableID      = 100
)

// 预定义入站（JSON 数组，元素为 sing-box inbound 对象）。
const tunPreset = `[
  {
    "type": "tun",
    "tag": "tun-in",
    "interface_name": "tun0",
    "mtu": 1500,
    "address": [
      "172.19.0.1/30",
      "2001:470:f9da:fdfa::1/64"
    ],
    "auto_route": true,
    "auto_redirect": true,
    "stack": "system",
    "endpoint_independent_nat": true
  }
]`

const tproxyPreset = `[
  {
    "type": "tproxy",
    "tag": "tproxy-in",
    "listen": "0.0.0.0",
    "listen_port": 22026,
    "tcp_fast_open": true
  }
]`

const redirTproxyPreset = `[
  {
    "type": "redirect",
    "tag": "rdr-in",
    "listen": "0.0.0.0",
    "listen_port": 22025,
    "tcp_fast_open": true
  },
  {
    "type": "tproxy",
    "tag": "tproxy-in",
    "listen": "0.0.0.0",
    "listen_port": 22026,
    "tcp_fast_open": true
  }
]`

const socksPreset = `[
  {
    "type": "mixed",
    "tag": "mixed-in",
    "listen": "0.0.0.0",
    "listen_port": 20080,
    "tcp_fast_open": true
  }
]`

// Default builds the built-in manager config, used for first-run installs and
// as the base for migration from the old .conf layout.
func Default() *ManagerConfig {
	return &ManagerConfig{
		Dirs: Dirs{
			ConfigDir: "/etc/singbox",
			DataDir:   "/var/lib/singbox",
		},
		Daemon: DaemonSettings{
			// 面板无鉴权：默认仅监听 IPv4（0.0.0.0），不暴露 IPv6。
			// 需要 IPv6 时显式设置：[::]:8082（仅 IPv6）或 :8082（双栈）。
			WebAddr:           "0.0.0.0:8082",
			CliSocket:         "/run/singbox-manager/singbox-manager.sock",
			ApplyDelayMs:      1000,
			ReconcileInterval: "5s",
		},
		Modes: map[string]*Mode{
			"tun": {
				Label:  "TUN",
				Unit:   "singbox@tun",
				Config: "config_tun.json",
				Routing: &Routing{
					RuleIndex:  9000,
					TableIndex: 2022,
				},
				Cleanup: &Cleanup{
					NftTables: []string{"inet sing-box"},
				},
				Preset: tunPreset,
			},
			"tproxy": {
				Label:  "TPROXY",
				Unit:   "singbox@tproxy",
				Config: "config_tproxy.json",
				Env: &Env{
					TproxyPort:    defaultTproxyPort,
					ExcludeGID:    defaultExcludeGID,
					RoutingMark:   defaultRoutingMark,
					Fwmark:        defaultFwmark,
					TableID:       defaultTableID,
					NftablesTable: "sing-box_tproxy4",
				},
				Preset: tproxyPreset,
			},
			"redir-tproxy": {
				Label:  "REDIR-TPROXY",
				Unit:   "singbox@redir-tproxy",
				Config: "config_redir-tproxy.json",
				Env: &Env{
					TproxyPort:    defaultTproxyPort,
					RedirectPort:  defaultRedirectPort,
					ExcludeGID:    defaultExcludeGID,
					RoutingMark:   defaultRoutingMark,
					Fwmark:        defaultFwmark,
					TableID:       defaultTableID,
					NftablesTable: "sing-box_redir_tproxy4",
				},
				Preset: redirTproxyPreset,
			},
			"socks": {
				Label:  "SOCKS",
				Unit:   "singbox@socks",
				Config: "config_socks.json",
				Preset: socksPreset,
			},
			// server 模式：配置文件由用户自行管理（config_server.json 需用户
			// 自己放入配置目录），管理器只负责启停与监控，config sync 跳过。
			"server": {
				Label:  "SERVER",
				Unit:   "singbox@server",
				Config: "config_server.json",
			},
		},
	}
}
