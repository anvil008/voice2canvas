package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type staticLookupResult uint8

const (
	staticLookupMissing staticLookupResult = iota
	staticLookupFound
	staticLookupUnsafe
)

var fingerprintedStaticAsset = regexp.MustCompile(`[-.][A-Za-z0-9_-]{8,}\.[A-Za-z0-9]+$`)

// normalizeStaticDir resolves the optional V2UI_STATIC_DIR bundle root. An
// empty value leaves static serving disabled; anything set but unusable is a
// startup error rather than a silently backend-only process.
func normalizeStaticDir(staticDir string) (string, error) {
	staticDir = strings.TrimSpace(staticDir)
	if staticDir == "" {
		return "", nil
	}
	absPath, err := filepath.Abs(staticDir)
	if err != nil {
		return "", fmt.Errorf("resolve V2UI_STATIC_DIR: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", fmt.Errorf("V2UI_STATIC_DIR is unusable: %w", err)
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", fmt.Errorf("stat V2UI_STATIC_DIR: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("V2UI_STATIC_DIR must be a directory, got %q", staticDir)
	}
	index, _, result := openStaticFile(resolvedPath, "index.html")
	if result != staticLookupFound {
		return "", fmt.Errorf("V2UI_STATIC_DIR must contain a readable regular index.html, got %q", staticDir)
	}
	if err := index.Close(); err != nil {
		return "", fmt.Errorf("close V2UI_STATIC_DIR index.html: %w", err)
	}
	return resolvedPath, nil
}

func newProductionStaticHandler(root string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if isBackendRoutePath(request.URL.Path) {
			http.NotFound(writer, request)
			return
		}
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		relativePath, ok := staticRequestPath(request)
		if !ok {
			http.NotFound(writer, request)
			return
		}
		file, info, result := openStaticFile(root, relativePath)
		if result == staticLookupFound {
			defer file.Close()
			serveStaticFile(writer, request, file, info, relativePath)
			return
		}
		if result == staticLookupUnsafe || staticPathLooksLikeAsset(relativePath) {
			http.NotFound(writer, request)
			return
		}

		index, indexInfo, indexResult := openStaticFile(root, "index.html")
		if indexResult != staticLookupFound {
			http.NotFound(writer, request)
			return
		}
		defer index.Close()
		serveStaticFile(writer, request, index, indexInfo, "index.html")
	})
}

// isBackendRoutePath lists the prefixes NewHandler registers, so a static
// bundle can never shadow an API, protocol, health, or debug route.
func isBackendRoutePath(value string) bool {
	for _, prefix := range []string{"/api", "/ws", "/healthz", "/debug"} {
		if value == prefix || strings.HasPrefix(value, prefix+"/") {
			return true
		}
	}
	return false
}

func staticRequestPath(request *http.Request) (string, bool) {
	if request == nil || request.URL == nil {
		return "", false
	}
	rawPath := request.URL.EscapedPath()
	if rawPath == "" {
		rawPath = "/"
	}
	decodedPath, err := url.PathUnescape(rawPath)
	if err != nil || !strings.HasPrefix(decodedPath, "/") || strings.ContainsRune(decodedPath, '\x00') || strings.Contains(decodedPath, `\`) {
		return "", false
	}
	for _, segment := range strings.Split(strings.TrimPrefix(decodedPath, "/"), "/") {
		if segment == ".." {
			return "", false
		}
	}
	relativePath := strings.TrimPrefix(path.Clean(decodedPath), "/")
	if relativePath == "." {
		relativePath = ""
	}
	return relativePath, true
}

func openStaticFile(root, relativePath string) (*os.File, os.FileInfo, staticLookupResult) {
	if relativePath == "" {
		return nil, nil, staticLookupMissing
	}
	currentPath := root
	segments := strings.Split(relativePath, "/")
	for index, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || strings.Contains(segment, `\`) {
			return nil, nil, staticLookupUnsafe
		}
		currentPath = filepath.Join(currentPath, filepath.FromSlash(segment))
		info, err := os.Lstat(currentPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil, staticLookupMissing
			}
			return nil, nil, staticLookupUnsafe
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, nil, staticLookupUnsafe
		}
		if index < len(segments)-1 {
			if !info.IsDir() {
				return nil, nil, staticLookupUnsafe
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return nil, nil, staticLookupUnsafe
		}
		file, err := os.Open(currentPath)
		if err != nil {
			return nil, nil, staticLookupUnsafe
		}
		return file, info, staticLookupFound
	}
	return nil, nil, staticLookupMissing
}

func serveStaticFile(writer http.ResponseWriter, request *http.Request, file *os.File, info os.FileInfo, relativePath string) {
	if relativePath == "index.html" {
		writer.Header().Set("Cache-Control", "no-cache")
	} else if fingerprintedStaticAsset.MatchString(path.Base(relativePath)) {
		writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeContent(writer, request, info.Name(), info.ModTime(), file)
}

func staticPathLooksLikeAsset(relativePath string) bool {
	if relativePath == "" {
		return false
	}
	if relativePath == "assets" || strings.HasPrefix(relativePath, "assets/") {
		return true
	}
	name := path.Base(relativePath)
	return path.Ext(name) != "" || strings.HasPrefix(name, ".")
}
