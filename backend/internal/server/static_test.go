package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigFromEnvLoadsProductionStaticSettings(t *testing.T) {
	root := newStaticFixture(t)
	t.Setenv("PORT", "")
	t.Setenv("V2UI_LISTEN_ADDR", " 127.0.0.1:9081 ")
	t.Setenv("V2UI_STATIC_DIR", "  "+root+"  ")

	cfg := ConfigFromEnv()
	if cfg.ListenAddr != "127.0.0.1:9081" {
		t.Fatalf("listen address = %q", cfg.ListenAddr)
	}
	if cfg.StaticDir != root {
		t.Fatalf("static directory = %q, want %q", cfg.StaticDir, root)
	}
	normalized, err := normalizeStaticDir(cfg.StaticDir)
	if err != nil {
		t.Fatalf("normalize static directory: %v", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if normalized != resolvedRoot {
		t.Fatalf("normalized static directory = %q, want %q", normalized, resolvedRoot)
	}
}

func TestStaticConfigurationRejectsInvalidValues(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "not-a-directory")
	if err := os.WriteFile(file, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	missingIndex := filepath.Join(root, "missing-index")
	if err := os.Mkdir(missingIndex, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, staticDir := range map[string]string{
		"missing static directory": filepath.Join(root, "missing"),
		"static path is a file":    file,
		"static index is absent":   missingIndex,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := normalizeStaticDir(staticDir); err == nil {
				t.Fatal("normalizeStaticDir succeeded")
			}
			if _, err := NewHandler(Config{StaticDir: staticDir}); err == nil {
				t.Fatal("NewHandler accepted an unusable static root")
			}
		})
	}

	if got, err := normalizeStaticDir("   "); err != nil || got != "" {
		t.Fatalf("unset static directory = (%q, %v)", got, err)
	}
}

func TestBackendOnlyDevelopmentWhenStaticRootIsUnset(t *testing.T) {
	handler, err := NewHandler(Config{})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("backend-only root status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestProductionStaticRoutesPreserveBackendPrecedence(t *testing.T) {
	t.Setenv("VOICE2CANVAS_DEBUG", "true")
	root := newStaticFixture(t)
	handler := newStaticTestHandler(t, root)

	for _, test := range []struct {
		path       string
		wantStatus int
	}{
		{path: "/healthz", wantStatus: http.StatusOK},
		{path: "/api/agents", wantStatus: http.StatusOK},
		{path: "/debug/cards", wantStatus: http.StatusOK},
		{path: "/ws", wantStatus: http.StatusBadRequest},
		{path: "/api/not-a-route", wantStatus: http.StatusNotFound},
	} {
		t.Run(test.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
			if recorder.Code != test.wantStatus {
				t.Fatalf("%s status = %d, want %d", test.path, recorder.Code, test.wantStatus)
			}
			if strings.Contains(recorder.Body.String(), "static-shadow") {
				t.Fatalf("%s was served from the static root: %q", test.path, recorder.Body.String())
			}
		})
	}
}

func TestProductionStaticHandlerServesAssetsAndSPAFallback(t *testing.T) {
	handler := newStaticTestHandler(t, newStaticFixture(t))

	root := httptest.NewRecorder()
	handler.ServeHTTP(root, httptest.NewRequest(http.MethodGet, "/", nil))
	if root.Code != http.StatusOK || root.Body.String() != "<html>spa-index</html>" {
		t.Fatalf("root response = (%d, %q)", root.Code, root.Body.String())
	}
	if got := root.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("root Cache-Control = %q, want no-cache", got)
	}

	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/assets/app-abcdef12.js", nil))
	if asset.Code != http.StatusOK || asset.Body.String() != "console.log('asset');" {
		t.Fatalf("asset response = (%d, %q)", asset.Code, asset.Body.String())
	}
	if got, want := asset.Header().Get("Cache-Control"), "public, max-age=31536000, immutable"; got != want {
		t.Fatalf("asset Cache-Control = %q, want %q", got, want)
	}

	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/assets/app-abcdef12.js", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD asset response = (%d, %q)", head.Code, head.Body.String())
	}
	if got, want := head.Header().Get("Cache-Control"), "public, max-age=31536000, immutable"; got != want {
		t.Fatalf("HEAD asset Cache-Control = %q, want %q", got, want)
	}

	fallback := httptest.NewRecorder()
	handler.ServeHTTP(fallback, httptest.NewRequest(http.MethodGet, "/dashboard/today", nil))
	if fallback.Code != http.StatusOK || fallback.Body.String() != "<html>spa-index</html>" {
		t.Fatalf("SPA fallback response = (%d, %q)", fallback.Code, fallback.Body.String())
	}
	if got := fallback.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("SPA fallback Cache-Control = %q, want no-cache", got)
	}

	missingAsset := httptest.NewRecorder()
	handler.ServeHTTP(missingAsset, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
	if missingAsset.Code != http.StatusNotFound {
		t.Fatalf("missing asset status = %d, want %d", missingAsset.Code, http.StatusNotFound)
	}

	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/dashboard/today", nil))
	if method.Code != http.StatusMethodNotAllowed || method.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("static POST response = (%d, Allow %q)", method.Code, method.Header().Get("Allow"))
	}
}

func TestProductionStaticHandlerRejectsTraversal(t *testing.T) {
	root := newStaticFixture(t)
	secret := filepath.Join(filepath.Dir(root), "outside.txt")
	if err := os.WriteFile(secret, []byte("do-not-serve"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := newProductionStaticHandler(root)
	for _, requestPath := range []string{"/%2e%2e/outside.txt", "/assets/%2e%2e/outside.txt"} {
		t.Run(requestPath, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, requestPath, nil))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("traversal status = %d, want %d", recorder.Code, http.StatusNotFound)
			}
			if strings.Contains(recorder.Body.String(), "do-not-serve") {
				t.Fatalf("traversal served outside content: %q", recorder.Body.String())
			}
		})
	}
}

func newStaticTestHandler(t *testing.T, root string) http.Handler {
	t.Helper()
	handler, err := NewHandler(Config{StaticDir: root})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func newStaticFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, contents := range map[string]string{
		"index.html":             "<html>spa-index</html>",
		"assets/app-abcdef12.js": "console.log('asset');",
		"healthz":                "static-shadow",
		"api/agents":             "static-shadow",
		"api/not-a-route":        "static-shadow",
		"debug/cards":            "static-shadow",
		"ws":                     "static-shadow",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}
