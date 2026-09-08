package browser

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type displayPolicyKey struct{}

type displayPolicy struct {
	mu                   sync.Mutex
	background, headless bool
	attention            map[proto.TargetTargetID]*rod.Page
}

func policyFor(p *rod.Page) *displayPolicy {
	if p == nil {
		return nil
	}
	policy, _ := p.Browser().GetContext().Value(displayPolicyKey{}).(*displayPolicy)
	return policy
}

// ManualVerificationPages returns only pages that this browser has already
// identified as needing a human. Callers check them before starting another task.
func (b *DouyinBrowser) ManualVerificationPages() []*rod.Page {
	b.display.mu.Lock()
	defer b.display.mu.Unlock()
	pages := make([]*rod.Page, 0, len(b.display.attention))
	for _, p := range b.display.attention {
		pages = append(pages, p)
	}
	return pages
}

func ManualVerificationPending(p *rod.Page) bool {
	policy := policyFor(p)
	if policy == nil {
		return false
	}
	policy.mu.Lock()
	defer policy.mu.Unlock()
	return policy.attention[p.TargetID] != nil
}

func forgetManualVerification(p *rod.Page) {
	if policy := policyFor(p); policy != nil {
		policy.mu.Lock()
		delete(policy.attention, p.TargetID)
		policy.mu.Unlock()
	}
}

// ResumeBackground is called only after the platform-specific checker confirms
// that this page no longer needs login/verification. It preserves the target,
// document, editor, and in-flight action; it never reloads or resubmits anything.
func ResumeBackground(p *rod.Page) error {
	policy := policyFor(p)
	if policy == nil {
		return nil
	}
	if policy.background {
		ctx, cancel := context.WithTimeout(p.GetContext(), 2*time.Second)
		defer cancel()
		var focus proto.EmulationSetFocusEmulationEnabled
		if !p.LoadState(&focus) || !focus.Enabled {
			if err := (proto.EmulationSetFocusEmulationEnabled{Enabled: true}).Call(p.Context(ctx)); err != nil {
				return err
			}
		}
		bounds, err := p.Context(ctx).GetWindow()
		if err != nil {
			return err
		}
		if bounds.WindowState != proto.BrowserWindowStateMinimized {
			if err := p.Context(ctx).SetWindow(&proto.BrowserBounds{WindowState: proto.BrowserWindowStateMinimized}); err != nil {
				return err
			}
			if err := waitWindowState(p.Context(ctx), proto.BrowserWindowStateMinimized); err != nil {
				return err
			}
		}
	}
	forgetManualVerification(p)
	return nil
}

// Native window transitions may finish after CDP acknowledges setWindowBounds
// (notably macOS minimize animations). Wait for the observed state to settle
// before allowing the next background operation, using the caller's deadline.
func waitWindowState(p *rod.Page, want proto.BrowserWindowState) error {
	var stableSince time.Time
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		bounds, err := p.GetWindow()
		if err != nil {
			return err
		}
		if bounds.WindowState != want {
			stableSince = time.Time{}
		} else {
			if stableSince.IsZero() {
				stableSince = time.Now()
			}
			if time.Since(stableSince) >= 150*time.Millisecond {
				return nil
			}
		}
		select {
		case <-p.GetContext().Done():
			return p.GetContext().Err()
		case <-ticker.C:
		}
	}
}

// ForgetClosedVerification clears stale attention only after the live browser
// confirms that the target no longer exists. A transport failure is not closure.
func ForgetClosedVerification(p *rod.Page) bool {
	result, err := (proto.TargetGetTargets{}).Call(p.Browser().Context(p.GetContext()))
	if err != nil {
		return false
	}
	for _, target := range result.TargetInfos {
		if target.TargetID == p.TargetID {
			return false
		}
	}
	forgetManualVerification(p)
	return true
}

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
	if policy := policyFor(p); policy != nil {
		policy.mu.Lock()
		alreadyShown := policy.attention[p.TargetID] != nil
		policy.attention[p.TargetID] = p.Context(context.Background())
		policy.mu.Unlock()
		if policy.headless || alreadyShown {
			return
		}
		if policy.background {
			_ = (proto.EmulationSetFocusEmulationEnabled{Enabled: false}).Call(p)
		}
	}
	if bounds, err := p.GetWindow(); err == nil && bounds.WindowState == proto.BrowserWindowStateMinimized {
		_ = p.SetWindow(&proto.BrowserBounds{WindowState: proto.BrowserWindowStateNormal})
	}
	_ = (proto.PageBringToFront{}).Call(p)
	// Unlike Page.Activate, the explicit call retains the bounded page context.
	_ = (proto.TargetActivateTarget{TargetID: p.TargetID}).Call(p)
	_ = waitWindowState(p, proto.BrowserWindowStateNormal)
}
