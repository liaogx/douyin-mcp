package browser

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/liaogx/douyin-mcp/configs"
)

func TestInputsDoNotWaitForAnimationFrames(t *testing.T) {
	if os.Getenv("DY_BROWSER_TESTS") != "1" {
		t.Skip("set DY_BROWSER_TESTS=1")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8">
<button id="send">Local fixture only</button><input id="text" value="old">
<div id="rich" contenteditable style="width:300px;height:40px">old rich text</div>
<div style="height:1200px"></div><div id="qr" style="width:80px;height:80px;background:black"></div>
<script>window.framesRequested=0;window.clicks=0;window.trusted=false;
window.requestAnimationFrame=()=>{framesRequested++;return 1};
send.onclick=e=>{clicks++;trusted=e.isTrusted};</script>`))
	}))
	defer server.Close()
	b, err := New(configs.BrowserConfig{Headless: true})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	p, err := b.NewPage(ctx, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	// Another tab must not strand operations on the retained page.
	other, err := b.NewPage(ctx, server.URL+"/other")
	if err != nil {
		t.Fatal(err)
	}
	defer ClosePage(other)
	p = p.Context(ctx)
	button, err := p.Element("#send")
	if err != nil {
		t.Fatal(err)
	}
	if err := Hover(button); err != nil {
		t.Fatal(err)
	}
	if err := Click(button); err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{"#text", "#rich"} {
		box, err := p.Element(selector)
		if err != nil {
			t.Fatal(err)
		}
		if err := SelectAllText(box); err != nil {
			t.Fatal(err)
		}
		if err := InputText(box, "中文 fixture"); err != nil {
			t.Fatal(err)
		}
		v, err := box.Eval(`()=>'value' in this?this.value:this.textContent`)
		if err != nil || v.Value.Str() != "中文 fixture" {
			t.Fatal("normal editing pipeline failed", selector, err)
		}
		if err := SelectAllText(box); err != nil {
			t.Fatal(err)
		}
		if err := PressKey(box, input.Backspace); err != nil {
			t.Fatal(err)
		}
		v, err = box.Eval(`()=>'value' in this?this.value:this.textContent`)
		if err != nil || v.Value.Str() != "" {
			t.Fatal("selection was not cleared", err)
		}
	}
	qr, err := p.Element("#qr")
	if err != nil {
		t.Fatal(err)
	}
	data, err := ScreenshotPNG(qr)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != 80 || img.Bounds().Dy() != 80 {
		t.Fatal("QR screenshot was not element-scoped", err)
	}
	r, err := p.Eval(`()=>clicks===1&&trusted&&framesRequested===0`)
	if err != nil || !r.Value.Bool() {
		t.Fatal("input was repeated, untrusted, or waited for a stalled frame", err)
	}
	// A covering dialog must not be clicked through, and the caller's short
	// deadline must win over both the retained tab and the default 10s cap.
	if _, err := p.Eval(`()=>{const e=document.createElement('div');e.style='position:fixed;inset:0;z-index:999;background:white';document.body.append(e);}`); err != nil {
		t.Fatal(err)
	}
	short, stop := context.WithTimeout(ctx, 350*time.Millisecond)
	start := time.Now()
	err = Click(button.Context(short))
	stop()
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 2*time.Second {
		t.Fatal("covered button ignored deadline", err)
	}
	r, err = p.Eval(`()=>clicks===1`)
	if err != nil || !r.Value.Bool() {
		t.Fatal("clicked through an overlay", err)
	}
}
