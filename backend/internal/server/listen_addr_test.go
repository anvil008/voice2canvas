package server

import (
	"testing"
)

func TestResolveListenAddrDefault(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("V2UI_LISTEN_ADDR", "")
	t.Setenv("VOICE2CANVAS_LISTEN_ADDR", "")

	if got := resolveListenAddr(); got != DefaultListenAddr {
		t.Fatalf("resolveListenAddr() = %q, want %q", got, DefaultListenAddr)
	}
	cfg := ConfigFromEnv()
	if cfg.ListenAddr != DefaultListenAddr {
		t.Fatalf("cfg.ListenAddr = %q, want %q", cfg.ListenAddr, DefaultListenAddr)
	}
}

func TestResolveListenAddrOverridePort(t *testing.T) {
	t.Setenv("VOICE2CANVAS_LISTEN_ADDR", "")
	t.Setenv("V2UI_LISTEN_ADDR", "")
	t.Setenv("PORT", "8083")

	if got := resolveListenAddr(); got != ":8083" {
		t.Fatalf("resolveListenAddr() = %q, want %q", got, ":8083")
	}
	cfg := ConfigFromEnv()
	if cfg.ListenAddr != ":8083" {
		t.Fatalf("cfg.ListenAddr = %q, want %q", cfg.ListenAddr, ":8083")
	}
}

func TestResolveListenAddrOverridePortWithColon(t *testing.T) {
	t.Setenv("VOICE2CANVAS_LISTEN_ADDR", "")
	t.Setenv("V2UI_LISTEN_ADDR", "")
	t.Setenv("PORT", ":9090")

	if got := resolveListenAddr(); got != ":9090" {
		t.Fatalf("resolveListenAddr() = %q, want %q", got, ":9090")
	}
}

func TestResolveListenAddrOverrideListenAddr(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("VOICE2CANVAS_LISTEN_ADDR", "")
	t.Setenv("V2UI_LISTEN_ADDR", "0.0.0.0:8083")

	if got := resolveListenAddr(); got != "0.0.0.0:8083" {
		t.Fatalf("resolveListenAddr() = %q, want %q", got, "0.0.0.0:8083")
	}
	cfg := ConfigFromEnv()
	if cfg.ListenAddr != "0.0.0.0:8083" {
		t.Fatalf("cfg.ListenAddr = %q, want %q", cfg.ListenAddr, "0.0.0.0:8083")
	}
}

func TestResolveListenAddrVoice2CanvasTakesPrecedence(t *testing.T) {
	t.Setenv("PORT", "8081")
	t.Setenv("V2UI_LISTEN_ADDR", "0.0.0.0:8082")
	t.Setenv("VOICE2CANVAS_LISTEN_ADDR", "0.0.0.0:8083")

	if got := resolveListenAddr(); got != "0.0.0.0:8083" {
		t.Fatalf("resolveListenAddr() = %q, want %q", got, "0.0.0.0:8083")
	}
	cfg := ConfigFromEnv()
	if cfg.ListenAddr != "0.0.0.0:8083" {
		t.Fatalf("cfg.ListenAddr = %q, want %q", cfg.ListenAddr, "0.0.0.0:8083")
	}
}

func TestConfigFromEnvModelsAndStaticDir(t *testing.T) {
	t.Setenv("VOICE2CANVAS_LIVE_MODEL", "custom-live")
	t.Setenv("V2UI_LIVE_MODEL", "fallback-live")
	t.Setenv("VOICE2CANVAS_CARD_MODEL", "custom-card")
	t.Setenv("V2UI_CARD_MODEL", "fallback-card")
	t.Setenv("VOICE2CANVAS_STATIC_DIR", "/custom/static")
	t.Setenv("V2UI_STATIC_DIR", "/fallback/static")

	cfg := ConfigFromEnv()
	if cfg.LiveModel != "custom-live" {
		t.Fatalf("cfg.LiveModel = %q, want %q", cfg.LiveModel, "custom-live")
	}
	if cfg.CardModel != "custom-card" {
		t.Fatalf("cfg.CardModel = %q, want %q", cfg.CardModel, "custom-card")
	}
	if cfg.StaticDir != "/custom/static" {
		t.Fatalf("cfg.StaticDir = %q, want %q", cfg.StaticDir, "/custom/static")
	}

	t.Setenv("VOICE2CANVAS_LIVE_MODEL", "")
	t.Setenv("VOICE2CANVAS_CARD_MODEL", "")
	t.Setenv("VOICE2CANVAS_STATIC_DIR", "")

	cfgFallback := ConfigFromEnv()
	if cfgFallback.LiveModel != "fallback-live" {
		t.Fatalf("cfgFallback.LiveModel = %q, want %q", cfgFallback.LiveModel, "fallback-live")
	}
	if cfgFallback.CardModel != "fallback-card" {
		t.Fatalf("cfgFallback.CardModel = %q, want %q", cfgFallback.CardModel, "fallback-card")
	}
	if cfgFallback.StaticDir != "/fallback/static" {
		t.Fatalf("cfgFallback.StaticDir = %q, want %q", cfgFallback.StaticDir, "/fallback/static")
	}
}

func TestValidateListenAddr(t *testing.T) {
	for _, tc := range []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{name: "empty defaults", addr: "", want: DefaultListenAddr},
		{name: "host and port", addr: "0.0.0.0:8083", want: "0.0.0.0:8083"},
		{name: "colon port", addr: ":8083", want: ":8083"},
		{name: "loopback", addr: "127.0.0.1:8083", want: "127.0.0.1:8083"},
		{name: "port without colon normalized", addr: "8083", want: ":8083"},
		{name: "invalid port range", addr: ":70000", wantErr: true},
		{name: "invalid non-numeric port", addr: ":port", wantErr: true},
		{name: "whitespace trimmed", addr: " :8083 ", want: ":8083"},
		{name: "embedded whitespace rejected", addr: ":80 83", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateListenAddr(tc.addr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateListenAddr(%q) succeeded, want error", tc.addr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateListenAddr(%q) failed: %v", tc.addr, err)
			}
			if got != tc.want {
				t.Fatalf("ValidateListenAddr(%q) = %q, want %q", tc.addr, got, tc.want)
			}
		})
	}
}
