package douyin

import (
	"context"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/liaogx/douyin-mcp/browser"
)

func TestWebBrowserCommentThenLikeReusesOnePage(t *testing.T) {
	s, br, ctx := webFixture(t)
	base := br.respond
	br.respond = func(h *rod.Hijack) bool {
		if h.Request.URL().Path == "/aweme/v1/web/comment/publish/" {
			h.Response.SetHeader("Content-Type", "application/json").SetBody(syntheticCommentAck)
			return true
		}
		return base(h)
	}
	// Deliberately no new DOM comment: only the bound server response can
	// confirm this synthetic submission. All requests are intercepted locally.
	br.html += `<script>window.send=e=>{submissions++;fetch('/aweme/v1/web/comment/publish/',{method:'POST',body:new URLSearchParams({aweme_id:'7665228646013364602',text:e.closest('.comment-input-inner-container').querySelector('[contenteditable]').textContent})});};</script>`
	post := "7665228646013364602"
	preview, err := s.Interact(ctx, "comment", &InteractionRequest{Post: post, Text: "合成测试评论"})
	if err != nil {
		t.Fatal(err)
	}
	pageID := s.detailPage.TargetID
	result, err := s.Interact(ctx, "comment", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
	if err != nil || !result.Success || result.Verification != "platform_response" || result.CommentID == "" {
		t.Fatalf("response-only comment not confirmed: %+v %v", result, err)
	}
	yes := true
	preview, err = s.Interact(ctx, "like", &InteractionRequest{Post: post, Liked: &yes})
	if err != nil {
		t.Fatal(err)
	}
	result, err = s.Interact(ctx, "like", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
	if err != nil || !result.Success || result.Verification != "platform_response" {
		t.Fatalf("reaction response not confirmed: %+v %v", result, err)
	}
	if br.pageCount != 1 || s.detailPage.TargetID != pageID || countSubmissions(t, s.detailPage.Context(ctx)) != 2 {
		t.Fatal("comment/like opened an extra page or duplicated submission")
	}
}

func TestWebBrowserHTTP200FailureOverridesOptimisticDOM(t *testing.T) {
	for _, kind := range []string{"comment", "like"} {
		t.Run(kind, func(t *testing.T) {
			s, br, ctx := webFixture(t)
			br.respond = func(h *rod.Hijack) bool {
				if h.Request.Method() != "POST" {
					return false
				}
				h.Response.SetHeader("Content-Type", "application/json").SetBody(`{"status_code":8}`)
				return true
			}
			br.html += `<script>const originalSend=send;window.send=e=>{const text=e.closest('.comment-input-inner-container').querySelector('[contenteditable]').textContent;originalSend(e);fetch('/aweme/v1/web/comment/publish/',{method:'POST',body:new URLSearchParams({aweme_id:'7665228646013364602',text})});};</script>`
			yes := true
			r := &InteractionRequest{Post: "7665228646013364602"}
			if kind == "comment" {
				r.Text = "合成测试评论"
			} else {
				r.Liked = &yes
			}
			preview, err := s.Interact(ctx, kind, r)
			if err != nil {
				t.Fatal(err)
			}
			result, err := s.Interact(ctx, kind, &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
			if err == nil || result == nil || result.Success || result.Stage != "unknown" {
				t.Fatalf("optimistic DOM/HTTP 200 caused false success: %+v %v", result, err)
			}
			_, _ = s.Interact(ctx, kind, &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
			if countSubmissions(t, s.detailPage.Context(ctx)) != 1 || br.pageCount != 1 {
				t.Fatal("unknown action retried or navigated")
			}
		})
	}
}

func TestWebBrowserOptimisticReactionWithoutAckIsUnknown(t *testing.T) {
	s, br, ctx := webFixture(t)
	br.html += `<script>window.ackReaction=()=>{};</script>`
	yes := true
	r := &InteractionRequest{Post: "7665228646013364602", Liked: &yes}
	preview, err := s.Interact(ctx, "like", r)
	if err != nil {
		t.Fatal(err)
	}
	limited, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	result, err := s.Interact(limited, "like", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
	if err == nil || result == nil || result.Success || result.Stage != "unknown" {
		t.Fatalf("color-only like was accepted: %+v %v", result, err)
	}
	preview, err = s.Interact(ctx, "like", r)
	if err != nil {
		t.Fatal(err)
	}
	result, err = s.Interact(ctx, "like", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
	if err == nil || result != nil && result.Success || countSubmissions(t, s.detailPage.Context(ctx)) != 1 || br.pageCount != 1 {
		t.Fatal("optimistic prior state incorrectly became an unchanged success")
	}
}

func TestWebBrowserSubmissionChallengeKeepsPageForHuman(t *testing.T) {
	s, br, ctx := webFixture(t)
	br.html += `<script>window.send=e=>{submissions++;const panel=document.createElement('div');panel.id='uc-second-verify';panel.innerHTML='<div class="second-verify-panel">请完成安全验证<button onclick="window.challengeClicks++">手动处理</button></div>';document.body.append(panel);};window.challengeClicks=0;</script>`
	preview, err := s.Interact(ctx, "comment", &InteractionRequest{Post: "7665228646013364602", Text: "合成测试评论"})
	if err != nil {
		t.Fatal(err)
	}
	p := s.detailPage.Context(ctx)
	var networkBefore proto.NetworkEnable
	hadNetwork := p.LoadState(&networkBefore)
	result, err := s.Interact(ctx, "comment", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
	if err == nil || result == nil || result.Success || result.Stage != "unknown" {
		t.Fatalf("challenge not preserved: %+v %v", result, err)
	}
	value, err := p.Eval(`()=>!!document.querySelector('#uc-second-verify') && window.challengeClicks===0 && window.submissions===1`)
	if err != nil || !value.Value.Bool() || br.pageCount != 1 {
		t.Fatal("verification was operated on, closed, or replaced")
	}
	var networkAfter proto.NetworkEnable
	if p.LoadState(&networkAfter) != hadNetwork {
		t.Fatal("temporary network listener remained enabled")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	started := time.Now()
	browser.ShowManualVerification(p.Context(cancelled))
	if time.Since(started) > time.Second {
		t.Fatal("focus helper ignored cancellation")
	}
	if _, err := p.Eval(`()=>document.querySelector('#uc-second-verify').remove()`); err != nil {
		t.Fatal(err)
	}
	_, _ = s.Interact(ctx, "comment", &InteractionRequest{ActionID: preview.ActionID, Confirm: true})
	if countSubmissions(t, p) != 1 {
		t.Fatal("completion of manual challenge caused a resend")
	}
}
