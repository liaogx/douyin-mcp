package douyin

import (
	"context"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/liaogx/douyin-mcp/browser"
)

// This marker belongs to this controller, not to Douyin. It is attached only
// after resolving the exact top-level or comment-owned composer via DOM.
const composerSelector = `[data-dy-mcp-composer="active"]`
const editorSelector = composerSelector + ` [contenteditable="true"]`

// A picker opened by this controller can otherwise cover the next reply
// button. New preparation is documented to replace the pending preview.
func dismissCommentPickers(p *rod.Page) error {
	open, err := p.Eval(`()=>{` + webDOMHelpers + `return !!one('` + composerSelector + ` .atBox-inner-container,` + composerSelector + ` .emoji-card-outer-container');}`)
	if err != nil || !open.Value.Bool() {
		return err
	}
	box, err := visibleElement(p, []string{editorSelector})
	if err != nil || box == nil {
		return err
	}
	if err := replaceCommentText(box, ""); err != nil {
		return err
	}
	return browser.PressKey(box, input.Escape)
}

func commentEditor(ctx context.Context, p *rod.Page, id string) (*rod.Element, error) {
	if err := ensureComments(ctx, p, id); err != nil {
		return nil, err
	}
	box, err := topCommentElement(p, id, `[contenteditable="true"]`)
	if err != nil {
		return nil, err
	}
	if box != nil {
		return markComposer(box)
	}
	trigger, err := topCommentElement(p, id, `._x9Gwl7G`)
	if err != nil {
		return nil, err
	}
	if trigger == nil {
		return nil, problem("editor_unavailable", "当前作品没有可用的评论编辑器", 409)
	}
	if err := clickOnce(trigger); err != nil {
		return nil, err
	}
	limited, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = poll(limited, 150*time.Millisecond, func() (bool, error) {
		var e error
		box, e = topCommentElement(p.Context(limited), id, `[contenteditable="true"]`)
		return box != nil, e
	})
	if err != nil {
		return nil, wrapTimeout(err, "评论编辑器未就绪")
	}
	return markComposer(box.Context(ctx))
}

func topCommentElement(p *rod.Page, id, selector string) (*rod.Element, error) {
	items, err := p.ElementsByJS(rod.Eval(`(id,selector)=>{`+webDOMHelpers+postDOM+`const root=postRoot(id);if(!root)return [];const top=all('.comment-input-inner-container',root).filter(e=>!e.closest('[data-e2e="comment-item"]'));return top.flatMap(e=>all(selector,e));}`, id, selector))
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	return exactlyOne(items, nil, "当前作品的顶层评论编辑器")
}

func markComposer(box *rod.Element) (*rod.Element, error) {
	_, err := box.Eval(`()=>{const root=this.closest('.comment-input-inner-container');if(!root)throw Error('Unknown composer root');document.querySelectorAll('[data-dy-mcp-composer]').forEach(e=>e.removeAttribute('data-dy-mcp-composer'));root.setAttribute('data-dy-mcp-composer','active');}`)
	return box, err
}

func replyEditor(ctx context.Context, p *rod.Page, id string, t commentTarget) (*rod.Element, error) {
	parent, err := findComment(p, id, t)
	if err != nil {
		return nil, err
	}
	items, err := parent.ElementsByJS(rod.Eval(`()=>{` + webDOMHelpers + commentDOM + `return own(this,'[contenteditable="true"]').filter(visible);}`))
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		button, err := replyButton(p, id, t)
		if err != nil {
			return nil, err
		}
		if err := clickOnce(button); err != nil {
			return nil, err
		}
	}
	limited, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var box *rod.Element
	err = poll(limited, 150*time.Millisecond, func() (bool, error) {
		items, e := parent.Context(limited).ElementsByJS(rod.Eval(`()=>{` + webDOMHelpers + commentDOM + `return own(this,'[contenteditable="true"]').filter(visible);}`))
		if e != nil {
			return false, e
		}
		if len(items) > 1 {
			return false, problem("ambiguous_editor", "目标评论中有多个回复编辑器", 409)
		}
		if len(items) == 1 {
			box = items[0]
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return nil, wrapTimeout(err, "所选评论的回复编辑器未就绪")
	}
	return markComposer(box.Context(ctx))
}

func replaceCommentText(box *rod.Element, value string) error {
	if _, err := box.Eval(`()=>{this.focus();const r=document.createRange();r.selectNodeContents(this);const s=window.getSelection();s.removeAllRanges();s.addRange(r);}`); err != nil {
		return err
	}
	if value == "" {
		return browser.PressKey(box, input.Backspace)
	}
	return browser.InputText(box, value)
}

func appendCommentText(box *rod.Element, value string) error {
	if value == "" {
		return nil
	}
	if _, err := box.Eval(`()=>{this.focus();const r=document.createRange();r.selectNodeContents(this);r.collapse(false);const s=window.getSelection();s.removeAllRanges();s.addRange(r);}`); err != nil {
		return err
	}
	return browser.InputText(box, value)
}

func replyButton(p *rod.Page, id string, t commentTarget) (*rod.Element, error) {
	el, err := findComment(p, id, t)
	if err != nil {
		return nil, err
	}
	items, err := el.ElementsByJS(rod.Eval(`()=>{` + webDOMHelpers + commentDOM + `return own(this,'.comment-item-stats-container .tFq3uJx3').filter(e=>visible(e)&&text(e)==='回复');}`))
	return exactlyOne(items, err, "目标评论的回复按钮")
}

type editorState struct {
	Text        string      `json:"text"`
	Placeholder string      `json:"placeholder"`
	Mentions    []string    `json:"mentions"`
	OwnerKey    string      `json:"owner_key"`
	Images      []string    `json:"images"`
	Files       []fileProof `json:"files"`
}

type fileProof struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
}

func readEditor(p *rod.Page) (editorState, error) {
	var state *editorState
	r, err := p.Eval(`()=>{` + webDOMHelpers + postDOM + `
const container=one('` + composerSelector + `'),box=container&&one('[contenteditable="true"]',container);if(!box)return null;
return {text:richText(box),placeholder:text(container.querySelector('.public-DraftEditorPlaceholder-inner')),
owner_key:container.closest('[data-e2e="comment-item"]')?.querySelector('[data-e2e="video-comment-more"] [id^="tooltip_"]')?.id||'',
mentions:[...box.querySelectorAll('.douyin_mention_word')].map(e=>e.textContent),
images:[...container.querySelectorAll('img')].filter(e=>!e.closest('a')&&!e.closest('.emoji-card-outer-container')&&!e.closest('.atBox-inner-container')).map(e=>e.currentSrc||e.src),
files:[...container.querySelectorAll('input[type="file"]')].flatMap(e=>[...e.files].map(f=>({name:f.name,size:f.size,modified:f.lastModified})))};
}`)
	if err != nil {
		return editorState{}, err
	}
	if err := r.Value.Unmarshal(&state); err != nil {
		return editorState{}, err
	}
	if state == nil {
		return editorState{}, problem("editor_changed", "评论编辑器已消失", 409)
	}
	return *state, nil
}

func verifyReplyPlaceholder(state editorState, t commentTarget) error {
	if state.OwnerKey != t.DOMKey || (state.Placeholder != "" && (!strings.Contains(state.Placeholder, "回复") || !strings.Contains(state.Placeholder, t.Comment.Author))) {
		return problem("reply_target_unknown", "无法确认编辑器正在回复所选评论，未发送", 409)
	}
	return nil
}

func commentSendButton(p *rod.Page) (*rod.Element, error) {
	// In the observed layout this is the initially hidden arrow; never use
	// generic “发送” selectors (which may address danmaku or direct messages).
	items, err := p.ElementsByJS(rod.Eval(`()=>{` + webDOMHelpers + `return all('` + composerSelector + ` .oWdMk9B9 > span.wchsYBpK.jfGCpJo0');}`))
	return exactlyOne(items, err, "评论发送按钮")
}
