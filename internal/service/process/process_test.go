//go:build container

package process

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"musicbox/internal/config"
	"musicbox/internal/service"
)

func TestManagerMaintainsSingleProcessPerUnit(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "sing-box")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 30\n"), 0755); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(root, "config")
	if err := os.Mkdir(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config_tproxy.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SINGBOX_BINARY", bin)
	cfg := &config.ManagerConfig{
		Dirs: config.Dirs{ConfigDir: configDir, DataDir: filepath.Join(root, "data")},
		Modes: map[string]*config.Mode{
			"tproxy": {Unit: "sing-box@tproxy", Config: "config_tproxy.json"},
		},
	}
	m := New(cfg)
	defer m.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	unit := "sing-box@tproxy.service"
	if err := m.Start(ctx, unit); err != nil {
		t.Fatal(err)
	}
	if got := m.ActiveState(ctx, unit); got != "active" {
		t.Fatalf("state after start = %q, want active", got)
	}
	if err := m.Start(ctx, unit); err != nil {
		t.Fatalf("second start failed: %v", err)
	}
	if err := m.Stop(ctx, unit); err != nil {
		t.Fatal(err)
	}
	if got := m.ActiveState(ctx, unit); got != "inactive" {
		t.Fatalf("state after stop = %q, want inactive", got)
	}
}

func TestManagerPublishesStateChanges(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "sing-box")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nsleep 30\n"), 0755); err != nil {
		t.Fatal(err)
	}
	configDir := filepath.Join(root, "config")
	if err := os.Mkdir(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config_socks.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SINGBOX_BINARY", bin)
	cfg := &config.ManagerConfig{
		Dirs: config.Dirs{ConfigDir: configDir, DataDir: filepath.Join(root, "data")},
		Modes: map[string]*config.Mode{
			"socks": {Unit: "sing-box@socks", Config: "config_socks.json"},
		},
	}
	m := New(cfg)
	defer m.Close()

	_ = m.ActiveState(context.Background(), "sing-box@socks")
	updates, _ := m.SubscribeStates(10 * time.Millisecond)
	select {
	case snapshot := <-updates:
		status := snapshot[service.Normalize("sing-box@socks")]
		if status == nil || status.ActiveState != "inactive" {
			t.Fatalf("initial state = %#v, want inactive", status)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial state")
	}
}
