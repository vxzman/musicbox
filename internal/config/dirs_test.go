package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalizeDirsRewritesProductionRelativePath(t *testing.T) {
	t.Setenv("MUSICBOX_CONFIG", "/opt/musicbox/manager.yaml")
	cfg := &ManagerConfig{Dirs: Dirs{ConfigDir: "etc-sing-box", DataDir: "/var/lib/singbox"}}
	if !canonicalizeDirs(cfg) {
		t.Fatal("expected production relative config_dir to be rewritten")
	}
	if cfg.Dirs.ConfigDir != "/etc/sing-box" {
		t.Fatalf("config_dir = %q, want /etc/sing-box", cfg.Dirs.ConfigDir)
	}
	if cfg.Dirs.DataDir != "/var/lib/sing-box" {
		t.Fatalf("data_dir = %q, want /var/lib/sing-box", cfg.Dirs.DataDir)
	}
}

func TestCanonicalizeDirsKeepsDevFixture(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "manager.yaml")
	if err := os.WriteFile(cfgPath, []byte("dirs: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "etc-sing-box"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MUSICBOX_CONFIG", cfgPath)
	cfg := &ManagerConfig{Dirs: Dirs{ConfigDir: "etc-sing-box", DataDir: "/var/lib/sing-box"}}
	if canonicalizeDirs(cfg) {
		t.Fatalf("dev fixture config_dir rewritten to %q", cfg.Dirs.ConfigDir)
	}
	if cfg.Dirs.ConfigDir != "etc-sing-box" {
		t.Fatalf("config_dir = %q, want etc-sing-box", cfg.Dirs.ConfigDir)
	}
}

func TestPruneRetiredModes(t *testing.T) {
	cfg := Default()
	cfg.Modes["server"] = &Mode{Label: "SERVER", Unit: "sing-box@server", Config: "config_server.json"}
	cfg.Modes["ep"] = &Mode{Label: "EP", Unit: "sing-box@ep", Config: "config_ep.json"}
	if !pruneRetiredModes(cfg) {
		t.Fatal("expected retired modes to be removed")
	}
	for _, name := range retiredModes {
		if _, ok := cfg.Modes[name]; ok {
			t.Fatalf("mode %s still present", name)
		}
	}
	for _, name := range BuiltinModes {
		if _, ok := cfg.Modes[name]; !ok {
			t.Fatalf("builtin mode %s missing", name)
		}
	}
}
