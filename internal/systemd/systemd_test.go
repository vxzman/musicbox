package systemd

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"singbox@tun":         "singbox@tun.service",
		"singbox@tproxy":      "singbox@tproxy.service",
		"singbox@tun.service": "singbox@tun.service",
		"foo.socket":          "foo.socket",
		"plain-service":       "plain-service.service",
		"":                    "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
