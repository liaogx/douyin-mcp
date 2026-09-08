package douyin

import (
	"context"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/liaogx/douyin-mcp/browser"
)

// CheckManualVerification checks only previously blocked, owned pages. A login
// QR can still be requested; an unresolved challenge stops unrelated operations.
func CheckManualVerification(ctx context.Context, br Browser) error {
	owner, ok := br.(interface{ ManualVerificationPages() []*rod.Page })
	if !ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	for _, page := range owner.ManualVerificationPages() {
		p := page.Context(ctx)
		info, err := p.Info()
		if err != nil {
			if browser.ForgetClosedVerification(p) {
				continue
			}
			return problem("needs_attention", "人工验证页面暂时无法读取，请保留专用窗口并完成验证", 409)
		}
		u, err := url.Parse(info.URL)
		if err != nil || u.Scheme != "https" || u.User != nil {
			return problem("needs_attention", "人工验证页面地址发生变化，请在原专用窗口处理", 409)
		}
		switch u.Host {
		case "www.douyin.com", "douyin.com":
			st, err := webSnapshot(p)
			if err != nil {
				return err
			}
			if st.Challenge {
				browser.ShowManualVerification(p)
				return problem("needs_attention", "原专用页面仍要求人工安全验证，已暂停其他操作", 409)
			}
			if st.Login || !st.Account {
				continue
			}
		case "creator.douyin.com":
			st, err := snapshot(p)
			if err != nil {
				return err
			}
			if manualBlock(st) {
				browser.ShowManualVerification(p)
				return problem("needs_attention", "原来的创作者页面仍要求人工验证，已暂停其他操作", 409)
			}
			if st.HasLogin {
				continue
			}
		default:
			return problem("needs_attention", "人工验证页面不在抖音官网，已暂停操作", 409)
		}
		if err := browser.ResumeBackground(p); err != nil {
			return err
		}
	}
	return nil
}
