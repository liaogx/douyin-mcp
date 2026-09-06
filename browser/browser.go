package browser

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
	"github.com/liaogx/douyin-mcp/configs"
	"net/url"
	"os"
	"runtime"
	"time"
)

// DouyinBrowser is owned by the serial service executor; not concurrently used.
type DouyinBrowser struct {
	root, session *rod.Browser
	launcher      *launcher.Launcher
	config        configs.BrowserConfig
	cancel        context.CancelFunc
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
	profile, err := os.MkdirTemp("", "douyin-mcp-browser-*")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	l := launcher.New().Context(ctx).Bin(bin).UserDataDir(profile).Headless(c.Headless).NoSandbox(c.NoSandbox).Leakless(false)
	if c.Proxy != "" {
		l.Proxy(c.Proxy)
	}
	endpoint, err := l.Launch()
	if err != nil {
		cancel()
		os.RemoveAll(profile)
		return nil, errors.New("专用浏览器启动失败，请检查浏览器路径和沙箱环境")
	}
	b := &DouyinBrowser{launcher: l, config: c, cancel: cancel}
	b.root = rod.New().ControlURL(endpoint)
	if err := b.root.Connect(); err != nil {
		b.Close()
		return nil, errors.New("无法连接专用浏览器")
	}
	if err := b.Reset(context.Background()); err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}

// Reset disposes the whole incognito context, not just its cookie backup.
// It does not revoke remote sessions or affect the user's normal Chrome.
func (b *DouyinBrowser) Reset(ctx context.Context) error {
	if b.session != nil {
		err := b.session.Context(ctx).Close()
		b.session = nil
		if err != nil {
			return fmt.Errorf("无法清除专用浏览器会话: %w", err)
		}
	}
	s, err := b.root.Context(ctx).Incognito()
	if err != nil {
		return err
	}
	b.session = s.Context(context.Background())
	return nil
}

func (b *DouyinBrowser) NewPage(ctx context.Context, target string) (*rod.Page, error) {
	if b.session == nil {
		return nil, errors.New("浏览器会话不可用，请重启服务")
	}
	p, err := b.session.Context(ctx).Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, err
	}
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
	if err := p.Navigate(target); err != nil {
		return nil, err
	}
	// Creator is an SPA; waiting for every third-party image/analytics resource
	// can hang despite an interactive login form. Business selectors poll below.
	if err := p.Wait(rod.Eval(`() => document.readyState === 'interactive' || document.readyState === 'complete'`)); err != nil {
		return nil, err
	}
	ok = true
	return p.Context(context.Background()), nil
}

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
		b.launcher.Cleanup()
	}
	if b.cancel != nil {
		b.cancel()
	}
	return err
}
