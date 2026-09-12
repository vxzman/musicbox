package service

import "testing"

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"sing-box@tun":         "sing-box@tun.service",
		"sing-box@tun.service": "sing-box@tun.service",
		"foo.socket":           "foo.socket",
		"":                     "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}
