package douyin

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/liaogx/douyin-mcp/browser"
	"github.com/liaogx/douyin-mcp/cookies"
	"net/url"
	"strings"
	"time"
)

// QR-only login: no phone number, verification-code field or messaging API.
type LoginService struct {
	browser Browser
	cookies cookies.Cookier
	page    *rod.Page
	homeURL string
}

func NewLoginServiceWithBrowser(br Browser, ck cookies.Cookier) *LoginService {
	return &LoginService{browser: br, cookies: ck, homeURL: CreatorHome}
}

func (s *LoginService) Restore(ctx context.Context) error {
	data, err := s.cookies.Load()
	if err != nil {
		return err
	}
	items, err := cookies.Decode(data, time.Now())
	if err != nil {
		return err
	}
	return s.browser.Restore(ctx, items)
}

func (s *LoginService) ensurePage(ctx context.Context) error {
	if s.page != nil {
		if _, err := s.page.Context(ctx).Info(); err == nil {
			return nil
		}
		browser.ClosePage(s.page)
		s.page = nil
	}
	p, err := s.browser.NewPage(ctx, s.homeURL)
	if err != nil {
		return err
	}
	s.page = p
	return nil
}

func sessionFingerprint(items []*proto.NetworkCookie) string {
	for _, name := range []string{"sessionid", "sessionid_ss"} {
		for _, c := range items {
			if c != nil && c.Name == name && cookies.AllowedDomain(c.Domain) && c.Value != "" {
				sum := sha256.Sum256([]byte(c.Value))
				return hex.EncodeToString(sum[:])
			}
		}
	}
	return ""
}

func authenticated(state pageState, fingerprint string) bool {
	u, err := url.Parse(state.URL)
	return err == nil && u.Hostname() == "creator.douyin.com" && strings.HasPrefix(u.Path, "/creator-micro/") &&
		!strings.Contains(u.Path, "login") && !state.HasLogin && !manualBlock(state) && fingerprint != "" &&
		(state.HasAvatar || state.Nickname != "" || strings.Contains(state.Text, "作品管理"))
}

func (s *LoginService) save(ctx context.Context) error {
	items, err := s.browser.Cookies(ctx)
	if err != nil {
		return err
	}
	data, err := cookies.Encode(items)
	if err != nil {
		return err
	}
	return s.cookies.Save(data)
}

func (s *LoginService) CheckLoginStatus(ctx context.Context) (*LoginResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := s.ensurePage(ctx); err != nil {
		return nil, err
	}
	var state pageState
	var fingerprint string
	err := poll(ctx, 300*time.Millisecond, func() (bool, error) {
		var err error
		state, err = snapshot(s.page.Context(ctx))
		if err != nil {
			return false, err
		}
		items, err := s.browser.Cookies(ctx)
		if err != nil {
			return false, err
		}
		fingerprint = sessionFingerprint(items)
		return authenticated(state, fingerprint) || state.HasLogin || manualBlock(state) || strings.Contains(state.Text, "登录"), nil
	})
	if err != nil {
		return nil, wrapTimeout(err, "无法判断登录状态；页面可能尚未加载或已改版")
	}
	if manualBlock(state) {
		return &LoginResult{Phase: PhaseNeedsAttention, Message: "抖音要求人工验证，请在专用浏览器或抖音 App 内处理；程序不会收集或提交验证码"}, nil
	}
	if authenticated(state, fingerprint) {
		if err := s.save(ctx); err != nil {
			return nil, fmt.Errorf("登录已完成，但凭证保存失败: %w", err)
		}
		return &LoginResult{Phase: PhaseLoggedIn, Success: true, Nickname: state.Nickname, Message: "已登录，凭证已安全写入本地；可关闭并重启本服务验证恢复"}, nil
	}
	return &LoginResult{Phase: PhaseQRCode, Message: "未登录，请获取二维码并使用抖音 App 扫码，随后再次检查登录状态"}, nil
}

func (s *LoginService) GetLoginQRCode(ctx context.Context) (*LoginResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := s.ensurePage(ctx); err != nil {
		return nil, err
	}
	page := s.page.Context(ctx)
	var qr *rod.Element
	clickedLogin, clickedQR := false, false
	err := poll(ctx, 400*time.Millisecond, func() (bool, error) {
		state, err := snapshot(page)
		if err != nil {
			return false, err
		}
		if manualBlock(state) {
			return false, problem("needs_attention", "抖音要求人工验证，请在专用浏览器或 App 中处理", 409)
		}
		items, err := s.browser.Cookies(ctx)
		if err != nil {
			return false, err
		}
		if authenticated(state, sessionFingerprint(items)) {
			return false, problem("already_logged_in", "当前已经登录，无需再次扫码", 409)
		}
		qr, err = visibleElement(page, qrSelectors)
		if err != nil {
			return false, err
		}
		if qr == nil && state.HasLogin {
			qr, err = contextualQRCode(page)
			if err != nil {
				return false, err
			}
		}
		if qr != nil {
			shape, err := qr.Shape()
			if err != nil {
				return false, err
			}
			box := shape.Box()
			if box != nil && box.Width >= 80 && box.Height >= 80 {
				return true, nil
			}
		}
		labels := []string{"扫码登录", "抖音扫码登录", "二维码登录"}
		if !clickedQR {
			el, err := textElement(page, labels, `button,a,span,div,[role="tab"]`)
			if err != nil {
				return false, err
			}
			if el != nil {
				if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
					return false, err
				}
				clickedQR = true
				return false, nil
			}
		}
		if !clickedLogin {
			el, err := textElement(page, []string{"登录", "立即登录"}, `button,a,span,div`)
			if err != nil {
				return false, err
			}
			if el != nil {
				if err := el.Click(proto.InputMouseButtonLeft, 1); err != nil {
					return false, err
				}
				clickedLogin = true
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, wrapTimeout(err, "未找到可扫描的登录二维码，请查看专用浏览器；不会切换到短信登录")
	}
	// Screenshot the rendered QR element: works for img, canvas and remote URLs,
	// and never fetches an arbitrary image URL from the server.
	png, err := qr.Screenshot(proto.PageCaptureScreenshotFormatPng, 100)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Phase: PhaseQRCode, QRPNG: png, QRImage: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), Message: "请使用抖音 App 扫码并在手机确认；完成后调用 check_login_status 保存登录状态"}, nil
}

func (s *LoginService) DeleteCookies(ctx context.Context) error {
	browser.ClosePage(s.page)
	s.page = nil
	// Clear the live session first, even if deleting the backup later fails.
	if err := s.browser.Reset(ctx); err != nil {
		return err
	}
	return s.cookies.Delete()
}

func (s *LoginService) Close() { browser.ClosePage(s.page); s.page = nil }

// The current creator login widget hashes all its CSS classes. Anchor to its
// nearby, visible Douyin-app scan instructions instead of a transient hash or
// the first canvas (the landing page also has several decorative canvases).
func contextualQRCode(page *rod.Page) (*rod.Element, error) {
	items, err := page.ElementsByJS(rod.Eval(`() => [...document.querySelectorAll('img,canvas')].filter(e=>{
 const r=e.getBoundingClientRect();
 if(!e.getClientRects().length || getComputedStyle(e).visibility==='hidden' || r.width<80 || r.width>320 || Math.abs(r.width-r.height)>5)return false;
 if(e.tagName==='IMG' && (!e.complete || e.naturalWidth<80 || !e.src.startsWith('data:image/')))return false;
 let parent=e.parentElement;
 for(let i=0;i<4&&parent;i++,parent=parent.parentElement){
   if(parent.tagName==='BODY'||parent.tagName==='HTML')break;
   const text=(parent.innerText||'').trim();
   if(text.length<160 && /抖音\s*APP/i.test(text) && /扫一扫/.test(text) && /如何扫码/.test(text))return true;
 }
 return false;
})`))
	if err != nil {
		return nil, err
	}
	// Wait for one unambiguous candidate; never choose the first of several.
	if len(items) > 1 {
		return nil, nil
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return nil, nil
}
