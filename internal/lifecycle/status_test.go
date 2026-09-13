package lifecycle

import (
	"context"
	"testing"
	"time"

	"musicbox/internal/config"
	"musicbox/internal/service"
)

type mockServiceManager struct {
	states map[string]string
}

func (m *mockServiceManager) Start(ctx context.Context, unit string) error { return nil }
func (m *mockServiceManager) Stop(ctx context.Context, unit string) error  { return nil }
func (m *mockServiceManager) ActiveState(ctx context.Context, unit string) string {
	if s, ok := m.states[unit]; ok {
		return s
	}
	return "inactive"
}
func (m *mockServiceManager) IsActive(ctx context.Context, unit string) bool {
	return m.ActiveState(ctx, unit) == "active"
}
func (m *mockServiceManager) SubscribeStates(interval time.Duration) (<-chan map[string]*service.UnitStatus, <-chan error) {
	return nil, nil
}
func (m *mockServiceManager) ActiveInstances(ctx context.Context, template string) []string {
	return nil
}
func (m *mockServiceManager) Close() {}

func TestStatus_InactiveRulesNotMissing(t *testing.T) {
	cfg := config.Default()
	sys := &mockServiceManager{
		states: map[string]string{
			"sing-box@tun":          "inactive",
			"sing-box@tproxy":       "inactive",
			"sing-box@redir-tproxy": "inactive",
			"sing-box@socks":        "inactive",
		},
	}
	mgr := New(cfg, sys)

	st := mgr.Status(context.Background())

	for _, mode := range []string{"tproxy", "redir-tproxy", "tun"} {
		ms, ok := st.Modes[mode]
		if !ok {
			t.Fatalf("mode %s missing in Status", mode)
		}
		if ms.Rules == "missing" {
			t.Errorf("mode %s is inactive, but Rules is 'missing'; want 'clean'", mode)
		}
		if ms.Rules != "clean" {
			t.Errorf("mode %s is inactive without rules applied, want Rules 'clean', got %q", mode, ms.Rules)
		}
	}
}
