package douyin

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-rod/rod"
)

type mentionTarget struct {
	Candidate             MentionCandidate
	PostID, Query, DOMKey string
	Expires               time.Time
}
type emojiTarget struct {
	Option         EmojiOption
	PostID, Source string
	Expires        time.Time
}

const pickerDOM = `
const sourceKey=raw=>{try{const u=new URL(raw,location.href);return u.origin+u.pathname;}catch{return '';}};
const mentionRows=()=>all('` + composerSelector + ` .atBox-inner-container .lgAE_oZa').flatMap(label=>{
 const e=label.closest('[id^="search_"]')||label.closest('.dTwsCivh');if(!e)return [];
 const row={element:e,key:e.id||'',name:text(label),avatar_url:safeURL(e.querySelector('img')?.src||'')};
 return row.name&&(row.key?/^search_\d+$/.test(row.key):!!row.avatar_url)?[row]:[];
});
`

func queryMentions(ctx context.Context, p *rod.Page, box *rod.Element, query string, replace bool) error {
	var err error
	if replace {
		err = replaceCommentText(box, "@"+query)
	} else {
		err = appendCommentText(box, " @"+query)
	}
	if err != nil {
		return err
	}
	// Debounced results must be stable, so initial suggestions cannot be
	// mistaken for results of the new query. User chooses a concrete ref.
	var previous string
	var stable time.Time
	limited, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	return poll(limited, 250*time.Millisecond, func() (bool, error) {
		if err := checkWebPage(p.Context(limited)); err != nil {
			return false, err
		}
		r, err := p.Context(limited).Eval(`()=>{` + webDOMHelpers + postDOM + pickerDOM + `return mentionRows().map(r=>[r.key,r.name,sourceKey(r.avatar_url)]);}`)
		if err != nil {
			return false, err
		}
		now := r.Value.JSON("", "")
		if now == "[]" {
			stable = time.Time{}
			return false, nil
		}
		if now != previous {
			previous = now
			stable = time.Now()
			return false, nil
		}
		return !stable.IsZero() && time.Since(stable) >= 1200*time.Millisecond, nil
	})
}

func (s *WebService) GetMentionCandidates(ctx context.Context, r *MentionRequest) (*MentionResult, error) {
	if r == nil || strings.TrimSpace(r.Query) == "" || utf8.RuneCountInString(r.Query) > 40 || strings.ContainsAny(r.Query, "@\x00\r\n") {
		return nil, problem("invalid_mention_query", "昵称查询需为 1–40 字，不含 @ 或换行", 400)
	}
	ctx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()
	s.discardInteraction()
	p, id, err := s.openPost(ctx, r.Post)
	if err != nil {
		return nil, err
	}
	box, err := commentEditor(ctx, p, id)
	if err != nil {
		return nil, err
	}
	if err := queryMentions(ctx, p, box, r.Query, true); err != nil {
		return nil, wrapTimeout(err, "未取得稳定的 @ 候选列表；未发送")
	}
	v, err := p.Eval(`()=>{` + webDOMHelpers + postDOM + pickerDOM + `return mentionRows().slice(0,30).map(r=>({picker_id:r.key,name:r.name,avatar_url:r.avatar_url}));}`)
	if err != nil {
		return nil, err
	}
	var candidates []MentionCandidate
	if err := v.Value.Unmarshal(&candidates); err != nil {
		return nil, err
	}
	now := time.Now()
	for k, t := range s.mentionRefs {
		if now.After(t.Expires) {
			delete(s.mentionRefs, k)
		}
	}
	if len(s.mentionRefs)+len(candidates) > 300 {
		return nil, problem("mention_ref_limit", "候选引用过多，请等待过期", 409)
	}
	for i := range candidates {
		ref, err := webNonce()
		if err != nil {
			return nil, err
		}
		candidates[i].Ref = ref
		s.mentionRefs[ref] = mentionTarget{Candidate: candidates[i], PostID: id, Query: r.Query, DOMKey: candidates[i].PickerID, Expires: now.Add(30 * time.Minute)}
	}
	return &MentionResult{Candidates: candidates, Message: "未发送。昵称可能重复，请核对头像和 picker_id 后选择 mention_ref；只输入 @昵称 不等于真实提及。新查询会替换未提交的编辑内容。"}, nil
}

func (s *WebService) selectMention(ctx context.Context, p *rod.Page, id string, box *rod.Element, ref string) (MentionCandidate, error) {
	t, ok := s.mentionRefs[ref]
	if !ok || t.PostID != id || time.Now().After(t.Expires) {
		return MentionCandidate{}, problem("mention_ref_expired", "@ 候选已过期或不属于此作品", 409)
	}
	if err := queryMentions(ctx, p, box, t.Query, false); err != nil {
		return t.Candidate, err
	}
	// Some candidates (including the current account) expose no picker ID.
	// Bind the exact name AND avatar source and require one unique match;
	// never fabricate a platform ID or choose a same-name entry by position.
	items, err := p.ElementsByJS(rod.Eval(`(key,name,avatar)=>{`+webDOMHelpers+postDOM+pickerDOM+`return mentionRows().filter(r=>r.key===key&&r.name===name&&sourceKey(r.avatar_url)===sourceKey(avatar)).map(r=>r.element);}`, t.DOMKey, t.Candidate.Name, t.Candidate.AvatarURL))
	el, err := exactlyOne(items, err, "@ 候选人")
	if err != nil {
		return t.Candidate, err
	}
	if err := clickOnce(el); err != nil {
		return t.Candidate, err
	}
	limited, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	err = poll(limited, 150*time.Millisecond, func() (bool, error) {
		v, e := p.Context(limited).Eval(`name=>{`+webDOMHelpers+`const box=one('`+editorSelector+`');return !!box && (box.textContent||'').replace(/\u00a0/g,' ').endsWith('@'+name+' ')&&!one('`+composerSelector+` .atBox-inner-container');}`, t.Candidate.Name)
		if e != nil {
			return false, e
		}
		return v.Value.Bool(), nil
	})
	return t.Candidate, wrapTimeout(err, "无法确认已从 @ 面板选中候选人，未发送")
}

func openEmojiPanel(ctx context.Context, p *rod.Page, id string) error {
	if err := verifyPost(p, id); err != nil {
		return err
	}
	v, err := p.Eval(`()=>{` + webDOMHelpers + `return !!one('` + composerSelector + ` .emoji-card-outer-container');}`)
	if err != nil {
		return err
	}
	if v.Value.Bool() {
		return nil
	}
	// The observed toolbar contains @, emoji, and send. The send control has
	// a distinct wchsYBpK class and can never be selected here.
	items, err := p.ElementsByJS(rod.Eval(`()=>{` + webDOMHelpers + `const buttons=all('` + composerSelector + ` .oWdMk9B9 > span');const plain=buttons.filter(e=>!e.classList.contains('wchsYBpK'));return plain.length===2?[plain[1]]:[];}`))
	el, err := exactlyOne(items, err, "表情面板入口")
	if err != nil {
		return err
	}
	if err := clickOnce(el); err != nil {
		return err
	}
	limited, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return poll(limited, 200*time.Millisecond, func() (bool, error) {
		r, e := p.Context(limited).Eval(`()=>{` + webDOMHelpers + `return all('` + composerSelector + ` .emoji-card-outer-container .uORo8cFf img').length>0;}`)
		if e != nil {
			return false, e
		}
		return r.Value.Bool(), nil
	})
}

func (s *WebService) GetEmojiOptions(ctx context.Context, r *PostRequest) (*EmojiResult, error) {
	if r == nil {
		return nil, problem("invalid_request", "缺少作品参数", 400)
	}
	ctx, cancel := context.WithTimeout(ctx, 50*time.Second)
	defer cancel()
	s.discardInteraction()
	p, id, err := s.openPost(ctx, r.Post)
	if err != nil {
		return nil, err
	}
	if _, err := commentEditor(ctx, p, id); err != nil {
		return nil, err
	}
	if err := openEmojiPanel(ctx, p, id); err != nil {
		return nil, err
	}
	v, err := p.Eval(`()=>{` + webDOMHelpers + postDOM + `return all('` + composerSelector + ` .emoji-card-outer-container .uORo8cFf img').slice(0,200).map(e=>({image_url:safeURL(e.src),label:e.alt||e.title||''})).filter(e=>e.image_url);}`)
	if err != nil {
		return nil, err
	}
	var options []EmojiOption
	if err := v.Value.Unmarshal(&options); err != nil {
		return nil, err
	}
	now := time.Now()
	for k, t := range s.emojiRefs {
		if now.After(t.Expires) {
			delete(s.emojiRefs, k)
		}
	}
	if len(s.emojiRefs)+len(options) > 1000 {
		return nil, problem("emoji_ref_limit", "表情引用过多，请等待过期", 409)
	}
	for i := range options {
		ref, err := webNonce()
		if err != nil {
			return nil, err
		}
		options[i].Ref = ref
		s.emojiRefs[ref] = emojiTarget{Option: options[i], PostID: id, Source: options[i].ImageURL, Expires: now.Add(30 * time.Minute)}
	}
	return &EmojiResult{Options: options, Message: "只读取已加载的表情，不发送。没有文字标签的表情返回预览地址与 emoji_ref；可将 ref 传入 emojis。也可直接在 text 中写普通 Unicode 表情。"}, nil
}

func (s *WebService) selectEmoji(ctx context.Context, p *rod.Page, id, ref string) error {
	t, ok := s.emojiRefs[ref]
	if !ok || t.PostID != id || time.Now().After(t.Expires) {
		return problem("emoji_ref_expired", "表情引用已过期或不属于此作品", 409)
	}
	if err := openEmojiPanel(ctx, p, id); err != nil {
		return err
	}
	before, err := readEditor(p)
	if err != nil {
		return err
	}
	items, err := p.ElementsByJS(rod.Eval(`source=>{`+webDOMHelpers+postDOM+pickerDOM+`return all('`+composerSelector+` .emoji-card-outer-container .uORo8cFf img').filter(e=>sourceKey(e.src)===sourceKey(source));}`, t.Source))
	el, err := exactlyOne(items, err, "表情选项")
	if err != nil {
		return err
	}
	if err := clickOnce(el); err != nil {
		return err
	}
	limited, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	return poll(limited, 150*time.Millisecond, func() (bool, error) {
		now, e := readEditor(p.Context(limited))
		return now.Text != before.Text || webDigest(now.Images) != webDigest(before.Images), e
	})
}
