package douyin

import (
	"context"
	"encoding/base64"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/liaogx/douyin-mcp/browser"
)

// WebService is used only inside the same serialized executor as publishing.
// It operates the rendered UI and can passively observe that UI's submission
// response. It never constructs or replays private network signing requests.
type WebService struct {
	browser                           Browser
	login                             *LoginService
	mediaRoot, dataDir                string
	loginPage, searchPage, detailPage *rod.Page
	searchKey, detailID               string
	searchFilterKey                   string
	searchSubmitted                   bool
	refs                              map[string]commentTarget
	mentionRefs                       map[string]mentionTarget
	emojiRefs                         map[string]emojiTarget
	active                            *interactionDraft
	pendingReactions                  map[string]bool // optimistic state on the retained detail page
}

func NewWebService(br Browser, login *LoginService, mediaRoot, dataDir string) *WebService {
	return &WebService{browser: br, login: login, mediaRoot: mediaRoot, dataDir: dataDir, refs: map[string]commentTarget{}, mentionRefs: map[string]mentionTarget{}, emojiRefs: map[string]emojiTarget{}}
}

func (s *WebService) Close() {
	s.closeInteraction()
	for _, p := range []*rod.Page{s.loginPage, s.searchPage, s.detailPage} {
		browser.ClosePage(p)
	}
	s.loginPage, s.searchPage, s.detailPage = nil, nil, nil
	s.searchKey, s.detailID = "", ""
	s.searchFilterKey = ""
	s.searchSubmitted = false
	s.pendingReactions = nil
	s.refs = map[string]commentTarget{}
	s.mentionRefs = map[string]mentionTarget{}
	s.emojiRefs = map[string]emojiTarget{}
}

const webDOMHelpers = `
const visible=e=>!!e && e.getClientRects().length>0 && getComputedStyle(e).visibility!=='hidden' && getComputedStyle(e).display!=='none';
const text=e=>(e?.innerText||'').trim();
const all=(selector,root=document)=>[...root.querySelectorAll(selector)].filter(visible);
const one=(selector,root=document)=>all(selector,root)[0];
`

type webState struct {
	URL       string `json:"url"`
	Login     bool   `json:"login"`
	Account   bool   `json:"account"`
	Challenge bool   `json:"challenge"`
	Nickname  string `json:"nickname"`
}

func webSnapshot(p *rod.Page) (webState, error) {
	var state webState
	r, err := p.Eval(`() => {` + webDOMHelpers + `
const header=document.querySelector('#douyin-header');
const loginPanel=one('#douyin_login_comp_flat_panel,#douyin-login-new-id,#login-panel-new');
const selfLoggedOut=location.pathname==='/user/self' && /^未登录/.test(text(one('[data-e2e="user-detail"]')));
const account=header && all('a[href]',header).find(e=>{try{return new URL(e.href).pathname==='/user/self' && !!e.querySelector('img');}catch{return false;}});
// Only platform login/challenge widgets count: a comment saying “扫码登录”
// or “拖动滑块” must never be interpreted as an account/security state.
const challenge=!!one('#uc-second-verify .second-verify-panel') || all('iframe').some(e=> /verifycenter\/captcha|rmc.bytedance.com\/verifycenter/.test(e.getAttribute('src')||'')) ||
 all('[id*="captcha"],[class*="captcha_verify"],[role="dialog"]').some(e=>/请完成下列验证|请完成安全验证|拖动.*滑块|拖动完成.*拼图|操作过于频繁/.test(text(e)));
return {url:location.href,login:!!loginPanel || selfLoggedOut || !!(header && all('button',header).find(e=>text(e)==='登录')),account:!!account && !selfLoggedOut,challenge,
 nickname:account?.querySelector('img')?.alt || ''};
}`)
	if err != nil {
		return state, err
	}
	err = r.Value.Unmarshal(&state)
	return state, err
}

func checkWebPage(p *rod.Page) error {
	state, err := webSnapshot(p)
	if err != nil {
		return err
	}
	u, err := url.Parse(state.URL)
	if err != nil || (u.Hostname() != "www.douyin.com" && u.Hostname() != "douyin.com") || u.Scheme != "https" {
		return problem("unexpected_page", "当前页面不再是抖音官网，已停止操作", 409)
	}
	if state.Challenge {
		browser.ShowManualVerification(p)
		return problem("needs_attention", "抖音要求人工安全验证，请在专用浏览器完成后重试；未绕过验证", 409)
	}
	if state.Login {
		return problem("login_required", "抖音网页版需要登录，请用 surface:web 获取二维码并检查状态", 401)
	}
	return nil
}

func (s *WebService) ensureLoginPage(ctx context.Context) error {
	if s.loginPage != nil {
		if _, err := s.loginPage.Context(ctx).Info(); err == nil {
			return nil
		}
		browser.ClosePage(s.loginPage)
		s.loginPage = nil
	}
	p, err := s.browser.NewPage(ctx, WebHome)
	if err != nil {
		return err
	}
	s.loginPage = p
	return nil
}

func (s *WebService) CheckLoginStatus(ctx context.Context) (*LoginResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := s.ensureLoginPage(ctx); err != nil {
		return nil, err
	}
	var st webState
	var stableSince time.Time
	err := poll(ctx, 400*time.Millisecond, func() (bool, error) {
		var err error
		st, err = webSnapshot(s.loginPage.Context(ctx))
		if err != nil {
			return false, err
		}
		if st.Login || st.Challenge {
			return true, nil
		}
		if !st.Account {
			stableSince = time.Time{}
			return false, nil
		}
		if stableSince.IsZero() {
			stableSince = time.Now()
		}
		return time.Since(stableSince) >= 4*time.Second, nil
	})
	if err != nil {
		return nil, wrapTimeout(err, "无法判断抖音网页版登录状态，请检查专用浏览器")
	}
	if st.Challenge {
		browser.ShowManualVerification(s.loginPage.Context(ctx))
		return &LoginResult{Phase: PhaseNeedsAttention, Message: "请在专用浏览器手动完成抖音安全验证"}, nil
	}
	items, err := s.browser.Cookies(ctx)
	if err != nil {
		return nil, err
	}
	if st.Account && !st.Login && sessionFingerprint(items) != "" {
		if err := s.login.save(ctx); err != nil {
			return nil, err
		}
		return &LoginResult{Phase: PhaseLoggedIn, Success: true, Nickname: st.Nickname, Message: "抖音网页版已登录，凭证已保存；创作者中心发布状态需单独检查"}, nil
	}
	return &LoginResult{Phase: PhaseQRCode, Message: "抖音网页版未登录，请用 surface:web 获取二维码，扫码后再次检查状态"}, nil
}

func (s *WebService) GetLoginQRCode(ctx context.Context) (*LoginResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if err := s.ensureLoginPage(ctx); err != nil {
		return nil, err
	}
	p := s.loginPage.Context(ctx)
	var qr *rod.Element
	clicked := false
	err := poll(ctx, 400*time.Millisecond, func() (bool, error) {
		st, err := webSnapshot(p)
		if err != nil {
			return false, err
		}
		if st.Challenge {
			browser.ShowManualVerification(p)
			return false, problem("needs_attention", "请在专用浏览器手工完成安全验证", 409)
		}
		items, err := s.browser.Cookies(ctx)
		if err != nil {
			return false, err
		}
		if st.Account && !st.Login && sessionFingerprint(items) != "" {
			return false, problem("already_logged_in", "抖音网页版已经登录，无需扫码", 409)
		}
		qr, err = visibleElement(p, []string{`#douyin_login_comp_scan_code img[aria-label="二维码"]`, `#douyin_login_comp_scan_code img[alt="二维码"]`, `#douyin_login_comp_scan_code img[class*="qrcode"]`, `#douyin_login_comp_scan_code canvas`})
		if err != nil {
			return false, err
		}
		if qr == nil && st.Login {
			items, err := p.ElementsByJS(rod.Eval(`() => {` + webDOMHelpers + `return all('#douyin_login_comp_scan_code img').filter(e=>e.naturalWidth>=100&&Math.abs(e.width-e.height)<5&&e.width>=80);}`))
			if err != nil {
				return false, err
			}
			if len(items) == 1 {
				qr = items[0]
			}
		}
		if qr != nil {
			return true, nil
		}
		if !clicked {
			el, err := textElement(p, []string{"登录", "立即登录"}, `#douyin-header button`)
			if err != nil {
				return false, err
			}
			if el != nil {
				if err := browser.Click(el); err != nil {
					return false, err
				}
				clicked = true
			}
		}
		return false, nil
	})
	if err != nil {
		return nil, wrapTimeout(err, "未找到网页版登录二维码，请查看专用浏览器")
	}
	png, err := browser.ScreenshotPNG(qr)
	if err != nil {
		return nil, err
	}
	return &LoginResult{Phase: PhaseQRCode, QRPNG: png, QRImage: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), Message: "请使用抖音 App 扫码，完成后调用 check_login_status 并指定 surface:web"}, nil
}
