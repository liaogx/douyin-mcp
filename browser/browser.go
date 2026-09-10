package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/liaogx/douyin-mcp/configs"
	"github.com/liaogx/douyin-mcp/internal/securefile"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// DouyinBrowser is owned by the serial service executor; not concurrently used.
type DouyinBrowser struct {
	root, session *rod.Browser
	launcher      *launcher.Launcher
	config        configs.BrowserConfig
	cancel        context.CancelFunc
	display       *displayPolicy
}

func FindBrowser(explicit string) (string, error) {
	if explicit != "" {
		i, err := os.Stat(explicit)
		if err != nil || i.IsDir() {
			return "", errors.New("指定的浏览器可执行文件不存在")
		}
		return explicit, nil
	}
	if runtime.GOOS == "darwin" {
		for _, p := range []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium"} {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	if p, ok := launcher.LookPath(); ok {
		return p, nil
	}
	return "", errors.New("未找到 Chrome/Chromium，请安装浏览器或通过 --bin 指定；不会自动下载浏览器")
}

func New(c configs.BrowserConfig) (*DouyinBrowser, error) {
	bin, err := FindBrowser(c.BinPath)
	if err != nil {
		return nil, err
	}
	if c.Proxy != "" {
		u, err := url.Parse(c.Proxy)
		if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5") || u.User != nil {
			return nil, errors.New("代理仅支持不含账号密码的 http/https/socks5 地址")
		}
	}
	profile := c.ProfileDir
	if profile == "" {
		profile, err = os.MkdirTemp("", "douyin-mcp-browser-*")
		if err != nil {
			return nil, err
		}
	} else {
		if err := ownedProfile(profile); err != nil {
			return nil, err
		}
	}
	display := &displayPolicy{background: c.Background && !c.Headless, headless: c.Headless, attention: map[proto.TargetTargetID]*rod.Page{}}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), displayPolicyKey{}, display))
	l := launcher.New().Context(ctx).Bin(bin).UserDataDir(profile).Headless(c.Headless).NoSandbox(c.NoSandbox).Leakless(false)
	if c.Proxy != "" {
		l.Proxy(c.Proxy)
	}
	endpoint, err := l.Launch()
	if err != nil {
		cancel()
		if c.ProfileDir == "" {
			os.RemoveAll(profile)
		}
		return nil, errors.New("专用浏览器启动失败，请检查浏览器路径和沙箱环境")
	}
	b := &DouyinBrowser{launcher: l, config: c, cancel: cancel, display: display}
	b.root = rod.New().Context(ctx).ControlURL(endpoint)
	if err := b.root.Connect(); err != nil {
		b.Close()
		return nil, errors.New("无法连接专用浏览器")
	}
	if c.ProfileDir != "" {
		b.session = b.root
		return b, nil
	}
	if err := b.Reset(context.Background()); err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}

// Reset disposes the dedicated persistent profile (or temporary incognito
// context in tests), not just its cookie backup.
// It does not revoke remote sessions or affect the user's normal Chrome.
func (b *DouyinBrowser) Reset(ctx context.Context) error {
	if b.config.ProfileDir != "" {
		if err := ctx.Err(); err != nil {
			return err
		}
		c := b.config
		if err := b.Close(); err != nil {
			return err
		}
		if err := ClearProfile(c.ProfileDir); err != nil {
			return err
		}
		fresh, err := New(c)
		if err != nil {
			return err
		}
		*b = *fresh
		return nil
	}
	if b.session != nil {
		err := b.session.Context(ctx).Close()
		b.session = nil
		if err != nil {
			return fmt.Errorf("无法清除专用浏览器会话: %w", err)
		}
		b.display.mu.Lock()
		clear(b.display.attention)
		b.display.mu.Unlock()
	}
	s, err := b.root.Context(ctx).Incognito()
	if err != nil {
		return err
	}
	b.session = s.Context(b.root.GetContext())
	return nil
}

func (b *DouyinBrowser) NewPage(ctx context.Context, target string) (*rod.Page, error) {
	if b.session == nil {
		return nil, errors.New("浏览器会话不可用，请重启服务")
	}
	// A retained tab's event watcher must live as long as the browser, not the
	// HTTP request that created it. p.Context(background) alone does NOT detach
	// rod's underlying session watcher from the original request context.
	targetInfo, err := b.createTarget(ctx)
	if err != nil && isTransientBrowserConnectionError(err) && b.config.ProfileDir != "" {
		if restartErr := b.restartPersistent(ctx); restartErr != nil {
			return nil, fmt.Errorf("专用浏览器连接已断开，重启持久化会话失败: %w", restartErr)
		}
		targetInfo, err = b.createTarget(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("创建专用标签页失败: %w", err)
	}
	// Bound attachment by this request without making the retained session's
	// watcher a child of it. Detach the cancellation hook once setup succeeds.
	pageCtx, cancelPage := context.WithCancel(b.session.GetContext())
	stopRequest := context.AfterFunc(ctx, cancelPage)
	p, err := b.session.Context(pageCtx).PageFromTarget(targetInfo.TargetID)
	detached := stopRequest()
	if err == nil && (!detached || ctx.Err() != nil) {
		err = ctx.Err()
		if err == nil {
			err = context.Canceled
		}
	}
	if err != nil {
		cancelPage()
		cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = (proto.TargetCloseTarget{TargetID: targetInfo.TargetID}).Call(b.session.Context(cleanup))
		return nil, err
	}
	// The CDP watcher must follow the retained page context, not the request
	// context below. Registering this after p.Context(ctx) would close the tab
	// as soon as the MCP request returned, defeating page/session reuse.
	context.AfterFunc(pageCtx, cancelPage)
	p = p.Context(ctx)
	ok := false
	defer func() {
		if !ok {
			ClosePage(p)
		}
	}()
	if b.config.Stealth {
		if _, err := p.EvalOnNewDocument(stealth.JS); err != nil {
			return nil, err
		}
	}
	if b.config.UserAgent != "" {
		if err := (proto.NetworkSetUserAgentOverride{UserAgent: b.config.UserAgent}).Call(p); err != nil {
			return nil, err
		}
	}
	if b.display.background {
		// Verify the browser honored the initial minimized state before loading
		// an account page. Older browsers must not silently expose a window.
		bounds, err := p.GetWindow()
		if err != nil || bounds.WindowState != proto.BrowserWindowStateMinimized {
			return nil, errors.New("浏览器不支持后台最小化窗口；请更新 Chrome，或显式使用 --background=false")
		}
		// Keep DOM focus/input and rendering active without restoring the OS
		// window. Minimizing alone can suspend interactive application code.
		if err := (proto.EmulationSetFocusEmulationEnabled{Enabled: true}).Call(p); err != nil {
			return nil, err
		}
	}
	if err := navigateWithRetry(ctx, p, target); err != nil {
		return nil, fmt.Errorf("页面导航失败: %w", err)
	}
	// Creator is an SPA; waiting for every third-party image/analytics resource
	// can hang despite an interactive login form. Business selectors poll below.
	if err := p.Wait(rod.Eval(`() => document.readyState === 'interactive' || document.readyState === 'complete'`)); err != nil {
		return nil, fmt.Errorf("等待页面可交互超时: %w", err)
	}
	ok = true
	return p.Context(context.Background()), nil
}

// navigateWithRetry handles transient proxy/browser connection closures at
// the page-opening boundary. It is deliberately limited to one retry and to
// transport errors; login, challenge, selector, and submission errors are
// never retried here.
func navigateWithRetry(ctx context.Context, p *rod.Page, target string) error {
	const attemptTimeout = 12 * time.Second
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, attemptTimeout)
		err := p.Context(attemptCtx).Navigate(target)
		cancel()
		if err == nil {
			return nil
		}
		last = err
		transientTimeout := errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil
		if attempt == 1 || ctx.Err() != nil || (!isTransientNavigationError(err) && !transientTimeout) {
			return err
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return last
}

func isTransientNavigationError(err error) bool {
	return hasErrorMarker(err,
		"ERR_CONNECTION_CLOSED",
		"ERR_CONNECTION_RESET",
		"ERR_CONNECTION_REFUSED",
		"ERR_NETWORK_CHANGED",
		"ERR_TIMED_OUT",
	)
}

func isTransientBrowserConnectionError(err error) bool {
	return isTransientNavigationError(err) || hasErrorMarker(err,
		"USE OF CLOSED NETWORK CONNECTION",
		"WEBSOCKET IS CLOSED",
		"CONNECTION IS CLOSED",
	)
}

func hasErrorMarker(err error, markers ...string) bool {
	if err == nil {
		return false
	}
	message := strings.ToUpper(err.Error())
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// restartPersistent recreates only the dedicated browser process. The
// persistent profile is intentionally left on disk, so cookies and the
// authenticated session can be reused after a transient CDP disconnect.
func (b *DouyinBrowser) restartPersistent(ctx context.Context) error {
	if b.config.ProfileDir == "" {
		return errors.New("临时浏览器会话不支持无损重启")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	_ = b.Close()
	fresh, err := New(b.config)
	if err != nil {
		return err
	}
	*b = *fresh
	return nil
}

func (b *DouyinBrowser) createTarget(ctx context.Context) (*proto.TargetCreateTargetResult, error) {
	req := proto.TargetCreateTarget{URL: "about:blank", BrowserContextID: b.session.BrowserContextID, Background: b.display.background}
	if !b.display.background {
		return req.Call(b.session.Context(ctx))
	}
	// windowState is a newer CDP field not present in Rod's generated structs.
	// Set it at creation, rather than showing a normal window and hiding it later.
	req.NewWindow = true
	req.Width, req.Height = intPtr(1440), intPtr(960)
	params := struct {
		proto.TargetCreateTarget
		WindowState string `json:"windowState"`
	}{req, "minimized"}
	data, err := b.session.Call(ctx, "", "Target.createTarget", params)
	if err != nil {
		return nil, err
	}
	var result proto.TargetCreateTargetResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func intPtr(n int) *int { return &n }

func (b *DouyinBrowser) Cookies(ctx context.Context) ([]*proto.NetworkCookie, error) {
	if b.session == nil {
		return nil, errors.New("浏览器会话不可用")
	}
	return b.session.Context(ctx).GetCookies()
}
func (b *DouyinBrowser) Restore(ctx context.Context, items []*proto.NetworkCookieParam) error {
	if len(items) == 0 {
		return nil
	}
	return b.session.Context(ctx).SetCookies(items)
}
func ClosePage(p *rod.Page) {
	if p == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = p.Context(ctx).Close()
	forgetManualVerification(p)
}
func (b *DouyinBrowser) Close() error {
	var err error
	if b.root != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err = b.root.Context(ctx).Close()
		cancel()
	}
	if b.launcher != nil {
		b.launcher.Kill()
		if b.config.ProfileDir == "" {
			b.launcher.Cleanup()
		}
	}
	if b.cancel != nil {
		b.cancel()
	}
	return err
}

func ownedProfile(dir string) error {
	// Never accept a daily Chrome directory or a broad path as a reset target.
	if !filepath.IsAbs(dir) || filepath.Base(dir) != "browser-profile" {
		return errors.New("浏览器资料必须是私有数据目录中的 browser-profile 子目录")
	}
	marker := filepath.Join(dir, ".douyin-mcp-owned")
	if info, err := os.Lstat(dir); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("专用浏览器资料目录无效")
		}
		data, err := securefile.Read(marker, 128)
		if err != nil {
			return err
		}
		if string(data) != "douyin-mcp dedicated browser profile\n" {
			return errors.New("拒绝接管未标记的浏览器资料目录")
		}
		return os.Chmod(dir, 0700)
	} else if !os.IsNotExist(err) {
		return err
	}
	return securefile.Write(marker, []byte("douyin-mcp dedicated browser profile\n"))
}

// ClearProfile can log out a stopped service without launching Chrome. Refuse
// unowned or still-running profiles, including a leftover singleton lock that
// needs human inspection after a crash. Never touch a daily Chrome profile.
func ClearProfile(dir string) error {
	if !filepath.IsAbs(dir) || filepath.Base(dir) != "browser-profile" {
		return errors.New("拒绝删除非专用浏览器资料目录")
	}
	if _, err := os.Lstat(dir); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err := ownedProfile(dir); err != nil {
		return err
	}
	for _, name := range []string{"SingletonLock", "SingletonSocket"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
			return errors.New("浏览器资料仍被锁定，请先关闭本工具的专用 Chrome 再清除登录")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return os.RemoveAll(dir)
}
