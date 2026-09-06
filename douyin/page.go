package douyin

import (
	"context"
	"errors"
	"github.com/go-rod/rod"
	"strings"
	"time"
)

// Prefer semantic selectors and fail on ambiguity instead of typing into an
// arbitrary input. These selectors are the only site-specific compatibility layer.
var titleSelectors = []string{`input[placeholder*="标题"]`, `textarea[placeholder*="标题"]`}
var bodySelectors = []string{`div.ProseMirror[contenteditable="true"]`, `[contenteditable="true"][data-placeholder*="描述"]`, `[contenteditable="true"][data-placeholder*="正文"]`, `[contenteditable="true"][data-placeholder*="作品"]`, `textarea[placeholder*="描述"]`, `textarea[placeholder*="作品简介"]`, `[contenteditable="true"]`}
var qrSelectors = []string{`img[class*="qrcode"]`, `img[class*="qr-code"]`, `[class*="qrcode"] img`, `[class*="qr-code"] img`, `[class*="qrcode"] canvas`, `[class*="qr-code"] canvas`, `canvas[class*="qrcode"]`, `canvas[class*="qr-code"]`, `img[alt*="二维码"]`}

func poll(ctx context.Context, interval time.Duration, f func() (bool, error)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		done, err := f()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func visibleElement(page *rod.Page, selectors []string) (*rod.Element, error) {
	for _, selector := range selectors {
		items, err := page.Elements(selector)
		if err != nil {
			return nil, err
		}
		var found *rod.Element
		for _, item := range items {
			visible, err := item.Visible()
			if err != nil {
				return nil, err
			}
			if !visible {
				continue
			}
			if found != nil {
				return nil, problem("ambiguous_page", "页面出现多个相同控件，请在浏览器中检查", 409)
			}
			found = item
		}
		if found != nil {
			return found, nil
		}
	}
	return nil, nil
}

func textElement(page *rod.Page, labels []string, selector string) (*rod.Element, error) {
	// One DOM query avoids hundreds of serial CDP round trips on the landing
	// page. Still require an exact, visible, unambiguous semantic match.
	items, err := page.ElementsByJS(rod.Eval(`(labels, selector) => {
 const visible=e=>e.getClientRects().length>0 && getComputedStyle(e).visibility!=='hidden';
 const candidates=[...document.querySelectorAll(selector)].filter(visible);
 for(const label of labels){
   const matches=candidates.filter(e=>(e.innerText||'').trim()===label && ![...e.querySelectorAll(selector)].some(c=>visible(c)&&(c.innerText||'').trim()===label));
   if(matches.length)return matches;
 }
 return [];
}`, labels, selector))
	if err != nil {
		return nil, err
	}
	if len(items) > 1 {
		return nil, problem("ambiguous_page", "页面存在多个相同按钮，未自动点击", 409)
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return nil, nil
}

type pageState struct {
	URL       string `json:"url"`
	Text      string `json:"text"`
	Nickname  string `json:"nickname"`
	HasAvatar bool   `json:"has_avatar"`
	HasLogin  bool   `json:"has_login"`
}

func snapshot(page *rod.Page) (pageState, error) {
	var state pageState
	r, err := page.Eval(`() => {
  const visible = e => !!e && e.getClientRects().length > 0 && getComputedStyle(e).visibility !== 'hidden';
  const first = selectors => selectors.flatMap(s => [...document.querySelectorAll(s)]).find(visible);
  const name = first(['[class*="user-name"]','[class*="userName"]','[class*="nickname"]']);
  const avatar = first(['[class*="user-avatar"]','[class*="userAvatar"]','[class*="header-user"]']);
  // Captions are user data, not login/captcha status. Exclude editable fields.
  const parts=[];
  const walker=document.createTreeWalker(document.body,NodeFilter.SHOW_TEXT);
  let node;
  while(node=walker.nextNode()){
    const p=node.parentElement;
    if(p && !p.closest('input,textarea,[contenteditable]') && visible(p))parts.push(node.textContent||'');
  }
  const text=parts.join('');
  return {url:location.href, text:text.slice(0,30000), nickname:name?.innerText?.trim() || '', has_avatar:!!avatar,
    has_login:/扫码登录|扫码登陆|请先登录|账号未登录/.test(text)};
}`)
	if err != nil {
		return state, err
	}
	err = r.Value.Unmarshal(&state)
	return state, err
}

func manualBlock(s pageState) bool {
	for _, marker := range []string{"请完成安全验证", "拖动滑块", "请进行身份验证", "请完成实名认证", "操作过于频繁", "请完成手机验证"} {
		if strings.Contains(s.Text, marker) {
			return true
		}
	}
	return false
}

func wrapTimeout(err error, message string) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return problem("timeout", message, 504)
	}
	return err
}
