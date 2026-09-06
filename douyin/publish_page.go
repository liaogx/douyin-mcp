package douyin

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"strings"
	"time"
)

func selectUpload(page *rod.Page, kind string) (*rod.Element, error) {
	items, err := page.Elements(`input[type="file"]`)
	if err != nil {
		return nil, err
	}
	var matches []*rod.Element
	for _, el := range items {
		attr, err := el.Attribute("accept")
		if err != nil {
			return nil, err
		}
		accept := ""
		if attr != nil {
			accept = strings.ToLower(*attr)
		}
		match := kind == "video" && (strings.Contains(accept, "video") || strings.Contains(accept, ".mp4"))
		match = match || kind == "image" && (strings.Contains(accept, "image") || strings.Contains(accept, ".png") || strings.Contains(accept, ".jpg"))
		if !match && !(len(items) == 1 && accept == "") {
			continue
		}
		v, err := el.Eval(`() => { let e=this.parentElement; while(e){ const s=getComputedStyle(e); if(s.display==='none'||s.visibility==='hidden')return false; e=e.parentElement; } return true; }`)
		if err != nil {
			return nil, err
		}
		if v.Value.Bool() {
			matches = append(matches, el)
		}
	}
	if len(matches) > 1 {
		return nil, problem("ambiguous_upload", "发现多个上传入口，未选择不确定的文件输入框", 409)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return nil, nil
}

func openUpload(ctx context.Context, page *rod.Page, kind string, paths []string) error {
	labels := []string{"发布视频"}
	if kind == "image" {
		labels = []string{"发布图文"}
	}
	clicked := false
	var input *rod.Element
	err := poll(ctx, 300*time.Millisecond, func() (bool, error) {
		state, err := snapshot(page)
		if err != nil {
			return false, err
		}
		if state.HasLogin {
			return false, problem("login_required", "上传页要求重新扫码登录", 401)
		}
		if manualBlock(state) {
			return false, problem("needs_attention", "上传页要求人工验证", 409)
		}
		if !clicked {
			tab, err := textElement(page, labels, `[role="tab"],button,a,span,div`)
			if err != nil {
				return false, err
			}
			if tab != nil {
				if err := tab.Click(proto.InputMouseButtonLeft, 1); err != nil {
					return false, err
				}
				clicked = true
			}
		}
		input, err = selectUpload(page, kind)
		if err != nil {
			return false, err
		}
		return input != nil, nil
	})
	if err != nil {
		return wrapTimeout(err, "未找到匹配的上传入口，请确认账号拥有该发布功能")
	}
	if len(paths) > 1 {
		multiple, err := input.Attribute("multiple")
		if err != nil {
			return err
		}
		if multiple == nil {
			return problem("multiple_images_unavailable", "当前上传入口不支持一次选择多图", 409)
		}
	}
	return input.SetFiles(paths)
}

func fillMetadata(ctx context.Context, page *rod.Page, req *PublishRequest) error {
	var title, body *rod.Element
	err := poll(ctx, 300*time.Millisecond, func() (bool, error) {
		if err := checkPageFailure(page); err != nil {
			return false, err
		}
		var err error
		title, err = visibleElement(page, titleSelectors)
		if err != nil {
			return false, err
		}
		body, err = visibleElement(page, bodySelectors)
		if err != nil {
			return false, err
		}
		return title != nil && body != nil, nil
	})
	if err != nil {
		return wrapTimeout(err, "上传后未出现标题和正文编辑框")
	}
	if err := title.SelectAllText(); err != nil {
		return err
	}
	if err := title.Input(req.Title); err != nil {
		return err
	}
	if _, err := body.Eval(`() => {
 this.focus();
 if (typeof this.select === 'function') { this.select(); return; }
 const range=document.createRange(); range.selectNodeContents(this);
 const selection=window.getSelection(); selection.removeAllRanges(); selection.addRange(range);
}`); err != nil {
		return err
	}
	if req.Description != "" {
		if err := body.Input(req.Description); err != nil {
			return err
		}
	} else {
		// Input("") does not delete an existing selection.
		if err := body.Type(input.Backspace); err != nil {
			return err
		}
	}
	return verifyMetadata(page, req)
}

func fieldText(el *rod.Element) (string, error) {
	r, err := el.Eval(`() => 'value' in this ? this.value : this.innerText`)
	if err != nil {
		return "", err
	}
	return r.Value.Str(), nil
}
func normalizedText(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\u00a0", " "))
}
func verifyMetadata(page *rod.Page, req *PublishRequest) error {
	for _, field := range []struct {
		selectors []string
		want      string
	}{{titleSelectors, req.Title}, {bodySelectors, req.Description}} {
		el, err := visibleElement(page, field.selectors)
		if err != nil {
			return err
		}
		if el == nil {
			return problem("editor_changed", "编辑框已消失，请重新准备预览", 409)
		}
		got, err := fieldText(el)
		if err != nil {
			return err
		}
		if normalizedText(got) != normalizedText(field.want) {
			return problem("metadata_changed", "页面中的标题或正文与已准备的内容不一致，未发布", 409)
		}
	}
	return nil
}

func checkPageFailure(page *rod.Page) error {
	s, err := snapshot(page)
	if err != nil {
		return err
	}
	if s.HasLogin {
		return problem("login_required", "登录已失效，请重新扫码", 401)
	}
	if manualBlock(s) {
		return problem("needs_attention", "抖音要求人工验证，请在专用浏览器或 App 内处理", 409)
	}
	// Inspect status UI only, not the user's caption which can contain these words.
	r, err := page.Eval(`() => [...document.querySelectorAll('[role="alert"],[class*="toast"],[class*="error"]')].some(e=>e.getClientRects().length && /上传失败|处理失败|格式不支持|发布失败/.test(e.innerText||''))`)
	if err != nil {
		return err
	}
	if r.Value.Bool() {
		return problem("platform_rejected", "抖音页面报告上传或发布失败，请在浏览器查看具体原因", 422)
	}
	return nil
}

func publishButton(page *rod.Page) (*rod.Element, error) {
	return textElement(page, []string{"发布", "立即发布", "发布作品"}, `button,[role="button"]`)
}

func waitReady(ctx context.Context, page *rod.Page) error {
	return poll(ctx, 400*time.Millisecond, func() (bool, error) {
		if err := checkPageFailure(page); err != nil {
			return false, err
		}
		button, err := publishButton(page)
		if err != nil {
			return false, err
		}
		if button == nil {
			return false, nil
		}
		r, err := button.Eval(`() => !this.disabled && this.getAttribute('aria-disabled')!=='true'`)
		if err != nil {
			return false, err
		}
		if !r.Value.Bool() {
			return false, nil
		}
		r, err = page.Eval(`() => [...document.querySelectorAll('[role="progressbar"],[class*="progress"]')].some(e=>e.getClientRects().length && (/上传中|正在上传|转码中|正在处理/.test(e.innerText||'') || (e.getAttribute('role')==='progressbar' && Number(e.getAttribute('aria-valuenow')||0)<Number(e.getAttribute('aria-valuemax')||100))))`)
		if err != nil {
			return false, err
		}
		return !r.Value.Bool(), nil
	})
}

func mediaProof(page *rod.Page) (string, error) {
	r, err := page.Eval(`() => ({files:[...document.querySelectorAll('input[type=file]')].flatMap(e=>[...e.files].map(f=>[f.name,f.size,f.lastModified])), previews:[...document.querySelectorAll('[class*="preview"] img,[class*="upload"] img,video')].filter(e=>e.getClientRects().length).map(e=>e.currentSrc||e.src||'')})`)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(r.Value)
	return string(b), err
}

// Success must come from status UI or an explicit post-publish redirect marker,
// never from matching the user's own title/caption or an old item in a list.
func publicationConfirmed(page *rod.Page) (bool, error) {
	r, err := page.Eval(`() => {
 const u=new URL(location.href);
 if(u.pathname.includes('/content/manage')&&u.searchParams.get('published')==='true')return true;
 return [...document.querySelectorAll('[role="alert"],[class*="toast"],[class*="message-content"],[class*="result-title"],[class*="success"]')].some(e=>e.getClientRects().length&&/^(作品)?(发布成功|提交成功)[！!。]?$/.test((e.innerText||'').trim()));
}`)
	if err != nil {
		return false, err
	}
	return r.Value.Bool(), nil
}

func verifyPublished(ctx context.Context, page *rod.Page) error {
	err := poll(ctx, 200*time.Millisecond, func() (bool, error) {
		ok, err := publicationConfirmed(page)
		if err != nil || ok {
			return ok, err
		}
		if err := checkPageFailure(page); err != nil {
			return false, err
		}
		return false, nil
	})
	if err != nil {
		return fmt.Errorf("发布结果未确认；请先查看创作者中心，勿自动重试: %w", err)
	}
	return nil
}
