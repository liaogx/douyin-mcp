package douyin

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"github.com/liaogx/douyin-mcp/browser"
	"github.com/liaogx/douyin-mcp/configs"
	"github.com/liaogx/douyin-mcp/cookies"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Every request in these tests is fulfilled locally BEFORE networking. The
// creator URL and cookie below are synthetic; no Douyin login or post occurs.
type fixtureBrowser struct {
	*browser.DouyinBrowser
	html      string
	respond   func(*rod.Hijack) bool
	pageCount int
}

func (b *fixtureBrowser) NewPage(ctx context.Context, target string) (*rod.Page, error) {
	b.pageCount++
	p, err := b.DouyinBrowser.NewPage(ctx, "about:blank")
	if err != nil {
		return nil, err
	}
	router := p.HijackRequests()
	if err := router.Add("*", "", func(h *rod.Hijack) {
		if b.respond != nil && b.respond(h) {
			return
		}
		h.Response.SetHeader("Content-Type", "text/html; charset=utf-8").SetBody(b.html)
	}); err != nil {
		browser.ClosePage(p)
		return nil, err
	}
	go router.Run()
	if err := p.Context(ctx).Navigate(target); err != nil {
		browser.ClosePage(p)
		return nil, err
	}
	if err := p.Context(ctx).WaitLoad(); err != nil {
		browser.ClosePage(p)
		return nil, err
	}
	return p, nil
}

const fixtureHTML = `<!doctype html><meta charset="utf-8">
<span class="user-name">本地测试账号</span><nav>作品管理</nav>
<button role="tab" onclick="pick('video')"><span>发布视频</span></button>
<button role="tab" onclick="pick('image')"><span>发布图文</span></button>
<section id="upload"><input type="file" accept="video/mp4" onchange="ready()"></section>
<section id="editor" hidden>
<input placeholder="填写标题"><div class="ProseMirror" contenteditable="true" style="min-height:80px;border:1px solid black"></div>
<button id="publish" onclick="submitPost()"><span>发布</span></button>
</section>
<script>
window.submissions=0;window.showSuccess=true;
function pick(kind){document.querySelector('#upload').innerHTML=kind==='video'?'<input type="file" accept="video/mp4" onchange="ready()">':'<input type="file" accept="image/png" multiple onchange="ready()">';}
function ready(){document.querySelector('#editor').hidden=false;}
function submitPost(){window.submissions++;if(window.showSuccess){const e=document.createElement('div');e.setAttribute('role','alert');e.textContent='发布成功';document.body.append(e);}}
</script>`

func newFixture(t *testing.T, html string) (*fixtureBrowser, context.Context) {
	t.Helper()
	if os.Getenv("DY_BROWSER_TESTS") != "1" {
		t.Skip("set DY_BROWSER_TESTS=1 to run local Chrome fixtures")
	}
	br, err := browser.New(configs.BrowserConfig{BinPath: os.Getenv("ROD_BROWSER_BIN"), Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = br.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	return &fixtureBrowser{DouyinBrowser: br, html: html}, ctx
}

func publisherFixture(t *testing.T) (*PublishService, *fixtureBrowser, context.Context) {
	t.Helper()
	br, ctx := newFixture(t, fixtureHTML)
	if err := br.Restore(ctx, []*proto.NetworkCookieParam{{Name: "sessionid", Value: "synthetic-local-fixture-session", Domain: ".douyin.com", Path: "/", Secure: true, HTTPOnly: true}}); err != nil {
		t.Fatal(err)
	}
	data, media := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "photo.png"), samplePNG(t), 0600); err != nil {
		t.Fatal(err)
	}
	// Only the container signature is validated locally, not video codec support.
	if err := os.WriteFile(filepath.Join(media, "clip.mp4"), append([]byte{0, 0, 0, 24}, []byte("ftypisom0000000000000000")...), 0600); err != nil {
		t.Fatal(err)
	}
	login := NewLoginServiceWithBrowser(br, cookies.NewFileCookiesWithPath(filepath.Join(data, "cookies.json")))
	t.Cleanup(login.Close)
	s := NewPublishService(br, login, media, data)
	t.Cleanup(s.Close)
	return s, br, ctx
}

func countSubmissions(t *testing.T, p *rod.Page) int {
	t.Helper()
	r, err := p.Eval(`() => window.submissions`)
	if err != nil {
		t.Fatal(err)
	}
	return r.Value.Int()
}

func TestBrowserPrepareConfirmAndPersistentIdempotency(t *testing.T) {
	for _, kind := range []string{"video", "image"} {
		t.Run(kind, func(t *testing.T) {
			s, br, ctx := publisherFixture(t)
			r := &PublishRequest{Title: "发布成功", Description: "扫码登录、拖动滑块、发布成功：这些是正文，不是页面状态。"}
			if kind == "video" {
				r.VideoPath = "clip.mp4"
			} else {
				r.ImagePaths = []string{"photo.png", "photo.png"}
			}
			preview, err := s.publish(ctx, kind, r)
			if err != nil {
				t.Fatal(err)
			}
			if preview.Success || preview.Stage != "ready" || len(preview.DraftID) != 32 {
				t.Fatalf("bad preview: %+v", preview)
			}
			p := s.active.page.Context(ctx)
			if countSubmissions(t, p) != 0 {
				t.Fatal("prepare clicked publish")
			}
			if got, err := publicationConfirmed(p); err != nil || got {
				t.Fatalf("caption mistaken for success: %v %v", got, err)
			}
			request := &PublishRequest{DraftID: preview.DraftID, Confirm: true}
			result, err := s.publish(ctx, kind, request)
			if err != nil {
				t.Fatal(err)
			}
			if !result.Success || result.Stage != "submitted" {
				t.Fatalf("not submitted: %+v", result)
			}
			if _, err = s.publish(ctx, kind, request); err != nil {
				t.Fatal(err)
			}
			// A new service has no active page, but loads the durable receipt.
			fresh := NewPublishService(br, s.login, s.mediaRoot, s.dataDir)
			if _, err = fresh.publish(ctx, kind, request); err != nil {
				t.Fatal(err)
			}
			if countSubmissions(t, p) != 1 {
				t.Fatal("duplicate submission")
			}
		})
	}
}

func TestBrowserChangedPreviewCannotPublish(t *testing.T) {
	for _, change := range []string{"metadata", "account", "media"} {
		t.Run(change, func(t *testing.T) {
			s, br, ctx := publisherFixture(t)
			preview, err := s.PublishVideo(ctx, &PublishRequest{Title: "确认内容", VideoPath: "clip.mp4"})
			if err != nil {
				t.Fatal(err)
			}
			p := s.active.page.Context(ctx)
			switch change {
			case "metadata":
				_, err = p.Eval(`() => document.querySelector('input[placeholder]').value='被修改'`)
			case "media":
				_, err = p.Eval(`() => document.querySelector('input[type=file]').value=''`)
			case "account":
				err = br.Restore(ctx, []*proto.NetworkCookieParam{{Name: "sessionid", Value: "another-synthetic-session", Domain: ".douyin.com", Path: "/", Secure: true, HTTPOnly: true}})
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.PublishVideo(ctx, &PublishRequest{DraftID: preview.DraftID, Confirm: true}); err == nil {
				t.Fatal("changed preview accepted")
			}
			if countSubmissions(t, p) != 0 {
				t.Fatal("changed preview was submitted")
			}
		})
	}
}

func TestBrowserUnknownSubmissionNeverRetries(t *testing.T) {
	s, br, ctx := publisherFixture(t)
	preview, err := s.PublishVideo(ctx, &PublishRequest{Title: "没有回执", VideoPath: "clip.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	p := s.active.page.Context(ctx)
	if _, err := p.Eval(`() => window.showSuccess=false`); err != nil {
		t.Fatal(err)
	}
	limited, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req := &PublishRequest{DraftID: preview.DraftID, Confirm: true}
	result, err := s.PublishVideo(limited, req)
	if err == nil || result == nil || result.Stage != "unknown" {
		t.Fatalf("wanted unknown, got %+v %v", result, err)
	}
	fresh := NewPublishService(br, s.login, s.mediaRoot, s.dataDir)
	if _, err = fresh.PublishVideo(ctx, req); err == nil {
		t.Fatal("incomplete receipt accepted as success")
	}
	if countSubmissions(t, p) != 1 {
		t.Fatal("unknown result retried")
	}
}

func TestBrowserCookiesRestoreResetAndQR(t *testing.T) {
	br, ctx := newFixture(t, `<!doctype html><meta charset="utf-8"><p>扫码登录</p><canvas class="qrcode" width="150" height="150"></canvas>`)
	ck := cookies.NewFileCookiesWithPath(filepath.Join(t.TempDir(), "cookies.json"))
	login := NewLoginServiceWithBrowser(br, ck)
	t.Cleanup(login.Close)
	qr, err := login.GetLoginQRCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(qr.QRPNG)); err != nil {
		t.Fatal("QR is not raw PNG", err)
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(qr.QRImage, "data:image/png;base64,"))
	if err != nil || !bytes.Equal(b, qr.QRPNG) {
		t.Fatal("QR base64 mismatch")
	}
	if _, err := login.page.Context(ctx).Eval(`() => {localStorage.setItem('synthetic','local');document.cookie='sessionid=synthetic-restore;path=/;secure';}`); err != nil {
		t.Fatal(err)
	}
	if err := login.save(ctx); err != nil {
		t.Fatal(err)
	}
	if err := br.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	if err := login.Restore(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := br.Cookies(ctx)
	if err != nil || sessionFingerprint(items) == "" {
		t.Fatalf("restore failed: %v", err)
	}
	if err := login.DeleteCookies(ctx); err != nil {
		t.Fatal(err)
	}
	items, err = br.Cookies(ctx)
	if err != nil || len(items) != 0 {
		t.Fatal("delete left browser cookies")
	}
	data, err := ck.Load()
	if err != nil || len(data) != 0 {
		t.Fatal("delete left cookie backup")
	}
	p, err := br.NewPage(ctx, CreatorHome)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.ClosePage(p)
	v, err := p.Context(ctx).Eval(`() => localStorage.getItem('synthetic') === null`)
	if err != nil || !v.Value.Bool() {
		t.Fatal("delete left local storage", err)
	}
}

func TestBrowserContextualQRCodeExcludesDecorations(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString(samplePNG(t))
	markup := `<meta charset="utf-8"><p>扫码登录</p><aside><canvas width="128" height="128"></canvas><img src="data:image/png;base64,` + encoded + `"></aside><section><div><div><img id="login-qr" src="data:image/png;base64,` + encoded + `"></div></div>如何扫码 打开「抖音APP」扫一扫</section>`
	br, ctx := newFixture(t, markup)
	page, err := br.NewPage(ctx, CreatorHome)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.ClosePage(page)
	qr, err := contextualQRCode(page.Context(ctx))
	if err != nil || qr == nil {
		t.Fatal("no contextual QR", err)
	}
	id, err := qr.Attribute("id")
	if err != nil || id == nil || *id != "login-qr" {
		t.Fatal("decoration matched as QR", err)
	}
	if _, err := page.Context(ctx).Eval(`() => document.body.append(document.querySelector('section').cloneNode(true))`); err != nil {
		t.Fatal(err)
	}
	qr, err = contextualQRCode(page.Context(ctx))
	if err != nil || qr != nil {
		t.Fatal("ambiguous QR selected", err)
	}
}

func TestLiveAnonymousQRCode(t *testing.T) {
	if os.Getenv("DY_LIVE_LOGIN_TEST") != "1" {
		t.Skip("explicit opt-in: anonymous Douyin page, no login or publication")
	}
	br, err := browser.New(configs.BrowserConfig{BinPath: os.Getenv("ROD_BROWSER_BIN"), Headless: true, Proxy: os.Getenv("DY_PROXY")})
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	login := NewLoginServiceWithBrowser(br, cookies.NewFileCookiesWithPath(filepath.Join(t.TempDir(), "cookies.json")))
	defer login.Close()
	qr, err := login.GetLoginQRCode(ctx)
	if err != nil {
		var e *Error
		if errors.As(err, &e) {
			t.Logf("anonymous live outcome: %s", e.Code)
		}
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(qr.QRPNG)); err != nil {
		t.Fatal(err)
	}
	t.Log("Anonymous live QR rendered as PNG; no account scanned and no content published")
}
