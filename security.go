package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"github.com/liaogx/douyin-mcp/configs"
	"github.com/liaogx/douyin-mcp/internal/securefile"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func authToken(c configs.Config) (string, error) {
	token := c.AuthToken
	path := filepath.Join(c.DataDir, "api-token")
	if token == "" {
		data, err := securefile.Read(path, 1024)
		if err != nil {
			return "", err
		}
		token = strings.TrimSpace(string(data))
		if len(data) == 0 {
			nonce := make([]byte, 32)
			if _, err := rand.Read(nonce); err != nil {
				return "", err
			}
			token = hex.EncodeToString(nonce)
			if err := securefile.Write(path, []byte(token+"\n")); err != nil {
				return "", err
			}
		}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9._~-]{32,256}$`).MatchString(token) {
		return "", errors.New("访问令牌格式无效，需 32–256 个字母、数字或 ._~- 字符")
	}
	return token, nil
}

func protect(c configs.Config, token string, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
		allowed[net.JoinHostPort(host, strconv.Itoa(c.Port))] = true
	}
	if ip := net.ParseIP(c.Host); ip != nil && !ip.IsUnspecified() {
		allowed[net.JoinHostPort(c.Host, strconv.Itoa(c.Port))] = true
	}
	expected := sha256.Sum256([]byte("Bearer " + token))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !allowed[strings.ToLower(r.Host)] {
			http.Error(w, "untrusted host", http.StatusForbidden)
			return
		}
		if origins := r.Header.Values("Origin"); len(origins) > 0 {
			u, err := url.Parse(origins[0])
			if len(origins) != 1 || err != nil || u.Scheme != "http" || u.Host != r.Host || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
				http.Error(w, "untrusted origin", http.StatusForbidden)
				return
			}
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "none" && site != "same-origin" {
			http.Error(w, "cross-site request rejected", http.StatusForbidden)
			return
		}
		// Public health reveals no account, browser or filesystem information.
		if r.Method == http.MethodGet && r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		got := sha256.Sum256([]byte(r.Header.Get("Authorization")))
		if len(r.Header.Values("Authorization")) != 1 || subtle.ConstantTimeCompare(got[:], expected[:]) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="douyin-mcp"`)
			http.Error(w, "authorization required", http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost && (r.URL.Path == "/mcp" || r.ContentLength != 0) {
			t, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || t != "application/json" {
				http.Error(w, "application/json required", http.StatusUnsupportedMediaType)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
		next.ServeHTTP(w, r)
	})
}
