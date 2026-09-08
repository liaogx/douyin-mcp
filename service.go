package main

import (
	"context"
	"errors"
	"github.com/liaogx/douyin-mcp/browser"
	"github.com/liaogx/douyin-mcp/configs"
	"github.com/liaogx/douyin-mcp/cookies"
	"github.com/liaogx/douyin-mcp/douyin"
	"path/filepath"
	"time"
)

type Operations interface {
	CheckLoginStatus(context.Context) (*douyin.LoginResult, error)
	GetLoginQRCode(context.Context) (*douyin.LoginResult, error)
	DeleteCookies(context.Context) error
	PublishVideo(context.Context, *douyin.PublishRequest) (*douyin.PublishResult, error)
	PublishImageText(context.Context, *douyin.PublishRequest) (*douyin.PublishResult, error)
}

// All operations share one bounded, cancelable executor. Separate requests must
// never mutate the same page/account concurrently.
type DouyinService struct {
	config    configs.Config
	gate      chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	browser   *browser.DouyinBrowser
	login     *douyin.LoginService
	publisher *douyin.PublishService
	web       *douyin.WebService
}

func NewDouyinService(c configs.Config) *DouyinService {
	ctx, cancel := context.WithCancel(context.Background())
	return &DouyinService{config: c, gate: make(chan struct{}, 1), ctx: ctx, cancel: cancel}
}

func (s *DouyinService) begin(ctx context.Context) (context.Context, func(), error) {
	if err := s.ctx.Err(); err != nil {
		return nil, nil, errors.New("服务已停止")
	}
	select {
	case s.gate <- struct{}{}:
	default:
		return nil, nil, &douyin.Error{Code: "busy", Message: "另一个浏览器操作正在进行，请稍后重试", Status: 409}
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.OperationTimeout)
	stop := context.AfterFunc(s.ctx, cancel)
	return ctx, func() { stop(); cancel(); <-s.gate }, nil
}

func (s *DouyinService) ensure(ctx context.Context) error {
	if s.browser != nil {
		return douyin.CheckManualVerification(ctx, s.browser)
	}
	c := s.config
	br, err := browser.New(configs.BrowserConfig{Headless: c.Headless, Background: c.Background, Stealth: c.Stealth, NoSandbox: c.NoSandbox, BinPath: c.BinPath, Proxy: c.Proxy, UserAgent: c.UserAgent, ProfileDir: filepath.Join(c.DataDir, "browser-profile")})
	if err != nil {
		return err
	}
	login := douyin.NewLoginServiceWithBrowser(br, cookies.NewFileCookiesWithPath(filepath.Join(c.DataDir, "cookies.json")))
	// The dedicated Chrome profile retains device-bound session state. Do not
	// overwrite newer profile cookies with an older JSON backup on startup.
	s.browser = br
	s.login = login
	s.publisher = douyin.NewPublishService(br, login, c.MediaRoot, c.DataDir)
	s.web = douyin.NewWebService(br, login, c.MediaRoot, c.DataDir)
	return nil
}

func (s *DouyinService) CheckLoginStatus(ctx context.Context) (*douyin.LoginResult, error) {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	if err := s.ensure(ctx); err != nil {
		return nil, err
	}
	return s.login.CheckLoginStatus(ctx)
}
func (s *DouyinService) GetLoginQRCode(ctx context.Context) (*douyin.LoginResult, error) {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	if err := s.ensure(ctx); err != nil {
		return nil, err
	}
	return s.login.GetLoginQRCode(ctx)
}
func (s *DouyinService) DeleteCookies(ctx context.Context) error {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	if s.browser == nil {
		if err := browser.ClearProfile(filepath.Join(s.config.DataDir, "browser-profile")); err != nil {
			return err
		}
		return cookies.NewFileCookiesWithPath(filepath.Join(s.config.DataDir, "cookies.json")).Delete()
	}
	s.publisher.Close()
	s.web.Close()
	return s.login.DeleteCookies(ctx)
}
func (s *DouyinService) publish(ctx context.Context, req *douyin.PublishRequest, video bool) (*douyin.PublishResult, error) {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	if err := s.ensure(ctx); err != nil {
		return nil, err
	}
	if video {
		return s.publisher.PublishVideo(ctx, req)
	}
	return s.publisher.PublishImageText(ctx, req)
}
func (s *DouyinService) PublishVideo(ctx context.Context, req *douyin.PublishRequest) (*douyin.PublishResult, error) {
	return s.publish(ctx, req, true)
}
func (s *DouyinService) PublishImageText(ctx context.Context, req *douyin.PublishRequest) (*douyin.PublishResult, error) {
	return s.publish(ctx, req, false)
}

func (s *DouyinService) Close() error {
	s.cancel()
	select {
	case s.gate <- struct{}{}:
	case <-time.After(15 * time.Second):
		return errors.New("浏览器操作未及时结束")
	}
	defer func() { <-s.gate }()
	if s.publisher != nil {
		s.publisher.Close()
	}
	if s.web != nil {
		s.web.Close()
	}
	if s.login != nil {
		s.login.Close()
	}
	if s.browser != nil {
		err := s.browser.Close()
		s.browser = nil
		return err
	}
	return nil
}
