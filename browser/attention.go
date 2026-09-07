package browser

import (
	"context"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ShowManualVerification only surfaces an already-owned page. Callers must
// first recognize a trusted platform challenge. It never opens another browser,
// interacts with the challenge, or turns a headless session into a visible one.
// OS focus is best effort; failure must not hide the original needs_attention.
func ShowManualVerification(p *rod.Page) {
	if p == nil {
		return
	}
	ctx, cancel := context.WithTimeout(p.GetContext(), 2*time.Second)
	defer cancel()
	p = p.Context(ctx)
	info, err := p.Info()
	if err != nil {
		return
	}
	u, err := url.Parse(info.URL)
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Host != "www.douyin.com" && u.Host != "douyin.com" && u.Host != "creator.douyin.com") {
		return
	}
	if bounds, err := p.GetWindow(); err == nil && bounds.WindowState == proto.BrowserWindowStateMinimized {
		_ = p.SetWindow(&proto.BrowserBounds{WindowState: proto.BrowserWindowStateNormal})
	}
	_ = (proto.PageBringToFront{}).Call(p)
	// Unlike Page.Activate, the explicit call retains the bounded page context.
	_ = (proto.TargetActivateTarget{TargetID: p.TargetID}).Call(p)
}
