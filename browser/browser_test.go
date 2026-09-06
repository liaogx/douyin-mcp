package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liaogx/douyin-mcp/configs"
)

func TestRefusesUnownedPersistentProfiles(t *testing.T) {
	parent := t.TempDir()
	for _, dir := range []string{parent, filepath.Join(parent, "Default"), filepath.Join(parent, "browser-profile")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := ownedProfile(dir); err == nil {
			t.Fatalf("unowned profile accepted: %s", dir)
		}
	}
}

func TestClearStoppedOwnedProfile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "browser-profile")
	if err := ClearProfile(dir); err != nil {
		t.Fatal(err)
	}
	if err := ownedProfile(dir); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(dir, "SingletonLock")
	if err := os.Symlink("synthetic-running-process", lock); err != nil {
		t.Fatal(err)
	}
	if err := ClearProfile(dir); err == nil {
		t.Fatal("cleared a locked profile")
	}
	if err := os.Remove(lock); err != nil {
		t.Fatal(err)
	}
	if err := ClearProfile(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatal("profile remains", err)
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := ClearProfile(dir); err == nil {
		t.Fatal("cleared unmarked profile")
	}
}

func TestPersistentBrowserRetainsStateAndResetClearsIt(t *testing.T) {
	if os.Getenv("DY_BROWSER_TESTS") != "1" {
		t.Skip("set DY_BROWSER_TESTS=1")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><p>Local persistence fixture</p>`))
	}))
	defer server.Close()
	c := configs.BrowserConfig{Headless: true, ProfileDir: filepath.Join(t.TempDir(), "browser-profile")}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	b, err := New(c)
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.NewPage(ctx, server.URL)
	if err != nil {
		b.Close()
		t.Fatal(err)
	}
	if _, err := p.Context(ctx).Eval(`()=>{localStorage.setItem('synthetic','local-only');document.cookie='synthetic=fixture;max-age=3600;path=/';}`); err != nil {
		b.Close()
		t.Fatal(err)
	}
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	b, err = New(c)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	p, err = b.NewPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r, err := p.Context(ctx).Eval(`()=>localStorage.getItem('synthetic')==='local-only'&&document.cookie.includes('synthetic=fixture')`)
	if err != nil || !r.Value.Bool() {
		t.Fatal("profile state not restored", err)
	}
	if err := b.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	p, err = b.NewPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r, err = p.Context(ctx).Eval(`()=>!localStorage.getItem('synthetic')&&!document.cookie.includes('synthetic=fixture')`)
	if err != nil || !r.Value.Bool() {
		t.Fatal("profile reset did not clear state", err)
	}
}

// Regression: the request creating a retained tab ends before the next SPA /
// cross-document navigation. Its CDP event watcher must not end with it.
func TestRetainedPageSurvivesRequestCancellation(t *testing.T) {
	if os.Getenv("DY_BROWSER_TESTS") != "1" {
		t.Skip("set DY_BROWSER_TESTS=1")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><title>retained page</title><p>request lifecycle</p>`))
	}))
	defer server.Close()
	b, err := New(configs.BrowserConfig{Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	first, cancelFirst := context.WithTimeout(context.Background(), 10*time.Second)
	p, err := b.NewPage(first, server.URL+"/first")
	cancelFirst()
	if err != nil {
		t.Fatal(err)
	}
	second, cancelSecond := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelSecond()
	if err := p.Context(second).Navigate(server.URL + "/second"); err != nil {
		t.Fatal(err)
	}
	v, err := p.Context(second).Eval(`()=>location.pathname`)
	if err != nil || v.Value.Str() != "/second" {
		t.Fatal("retained tab lost event context", err)
	}
	q, err := b.NewPage(second, server.URL+"/third")
	if err != nil {
		t.Fatal(err)
	}
	defer ClosePage(q)
	v, err = q.Context(second).Eval(`()=>location.pathname`)
	if err != nil || v.Value.Str() != "/third" {
		t.Fatal("second request could not open a page", err)
	}
}
