package douyin

import (
	"os"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/liaogx/douyin-mcp/browser"
)

// Runs in a separate, local synthetic profile. Only the simulated challenge
// briefly restores its own window; no live platform requests are sent.
func TestWebBrowserBackgroundManualHandoffKeepsDocument(t *testing.T) {
	if os.Getenv("DY_BACKGROUND_BROWSER_TESTS") != "1" {
		t.Skip("set DY_BROWSER_TESTS=1 DY_BACKGROUND_BROWSER_TESTS=1")
	}
	s, br, ctx := webFixture(t)
	preview, err := s.Interact(ctx, "comment", &InteractionRequest{Post: "7665228646013364602", Text: "合成测试评论"})
	if err != nil {
		t.Fatal(err)
	}
	p := s.detailPage.Context(ctx)
	id := p.TargetID
	assertWindow := func(want proto.BrowserWindowState) {
		t.Helper()
		bounds, err := p.GetWindow()
		if err != nil || bounds.WindowState != want {
			t.Fatalf("window state: %+v, want %s, error %v", bounds, want, err)
		}
	}
	assertWindow(proto.BrowserWindowStateMinimized)
	// A new background tab must not restore an existing window either.
	q, err := br.DouyinBrowser.NewPage(ctx, "about:blank")
	if err != nil {
		t.Fatal(err)
	}
	qb, err := q.Context(ctx).GetWindow()
	if err != nil || qb.WindowState != proto.BrowserWindowStateMinimized {
		t.Fatal("new page was not minimized", err)
	}
	browser.ClosePage(q)
	if _, err := p.Eval(`()=>{window.handoffMarker='same-document';window.challengeClicks=0;const e=document.createElement('div');e.id='uc-second-verify';e.innerHTML='<div class="second-verify-panel">请完成安全验证<button onclick="window.challengeClicks++">手动验证</button></div>';document.body.append(e);}`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Interact(ctx, "comment", &InteractionRequest{ActionID: preview.ActionID, Confirm: true}); err == nil {
		t.Fatal("verification did not block submission")
	}
	assertWindow(proto.BrowserWindowStateNormal)
	if !browser.ManualVerificationPending(p) || len(br.ManualVerificationPages()) != 1 {
		t.Fatal("handoff not tracked across request contexts")
	}
	if err := CheckManualVerification(ctx, br); err == nil {
		t.Fatal("unrelated operations were allowed during verification")
	}
	state, err := readEditor(p)
	if err != nil || state.Text != preview.Text || countSubmissions(t, p) != 0 {
		t.Fatal("handoff changed the draft or submitted it", err)
	}
	value, err := p.Eval(`()=>window.handoffMarker==='same-document'&&window.challengeClicks===0`)
	if err != nil || !value.Value.Bool() {
		t.Fatal("challenge was replaced or operated on", err)
	}
	// Removing this LOCAL fixture models completion by a human, not solving
	// any real challenge. The service only observes the changed platform UI.
	if _, err := p.Eval(`()=>document.querySelector('#uc-second-verify').remove()`); err != nil {
		t.Fatal(err)
	}
	if err := CheckManualVerification(ctx, br); err != nil {
		t.Fatal(err)
	}
	assertWindow(proto.BrowserWindowStateMinimized)
	if s.detailPage.TargetID != id || len(br.ManualVerificationPages()) != 0 {
		t.Fatal("completion lost the page or kept an obsolete handoff")
	}
	result, err := s.Interact(ctx, "comment", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
	if err != nil || !result.Success {
		t.Fatalf("background comment failed: %+v %v", result, err)
	}
	assertWindow(proto.BrowserWindowStateMinimized)
	yes := true
	preview, err = s.Interact(ctx, "like", &InteractionRequest{Post: "7665228646013364602", Liked: &yes})
	if err != nil {
		t.Fatal(err)
	}
	assertWindow(proto.BrowserWindowStateMinimized)
	result, err = s.Interact(ctx, "like", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
	if err != nil || !result.Success {
		t.Fatalf("background like failed: %+v %v", result, err)
	}
	assertWindow(proto.BrowserWindowStateMinimized)
	if br.pageCount != 1 || s.detailPage.TargetID != id || countSubmissions(t, p) != 2 {
		t.Fatal("resuming/comment/like created or submitted an extra page")
	}
}

func TestWebBrowserFailedPreparationPreservesVerification(t *testing.T) {
	s, br, ctx := webFixture(t)
	// The upload picker raises a simulated verification during preparation.
	br.html += `<script>document.querySelector('input[type=file]').addEventListener('change',()=>{const e=document.createElement('div');e.id='uc-second-verify';e.innerHTML='<div class="second-verify-panel">请完成安全验证</div>';document.body.append(e);});</script>`
	_, err := s.Interact(ctx, "comment", &InteractionRequest{Post: "7665228646013364602", Text: "合成测试评论", ImagePath: "photo.png"})
	if err == nil || s.detailPage == nil || !browser.ManualVerificationPending(s.detailPage) {
		t.Fatal("failed preparation closed or discarded the challenge", err)
	}
	p := s.detailPage.Context(ctx)
	els, err := p.ElementsByJS(rod.Eval(`()=>[...document.querySelectorAll('#uc-second-verify')]`))
	if err != nil || len(els) != 1 || countSubmissions(t, p) != 0 {
		t.Fatal("challenge page was lost or preparation submitted", err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := CheckManualVerification(ctx, br); err != nil || len(br.ManualVerificationPages()) != 0 {
		t.Fatal("a confirmed closed target permanently blocked the service", err)
	}
}
