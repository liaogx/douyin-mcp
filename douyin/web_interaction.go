package douyin

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/go-rod/rod"
	"github.com/liaogx/douyin-mcp/browser"
)

type commentTarget struct {
	DOMKey                             string
	ParentDOMKey                       string
	Comment                            Comment
	PostID, Signature, ParentSignature string
	Expires                            time.Time
}

type interactionDraft struct {
	id, kind, postID, fingerprint, proof string
	postProof, actorURL                  string
	target                               *commentTarget
	request                              InteractionRequest
	page                                 *rod.Page
	media                                *stagedMedia
	result                               *InteractionResult
	attempted                            bool
}

func (s *WebService) Interact(ctx context.Context, kind string, r *InteractionRequest) (*InteractionResult, error) {
	if err := validateInteraction(kind, r); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if r.Confirm {
		return s.confirmInteraction(ctx, kind, r.ActionID)
	}
	return s.prepareInteraction(ctx, kind, r)
}

func postContentProof(p *rod.Page, id string) (string, error) {
	if err := verifyPost(p, id); err != nil {
		return "", err
	}
	r, err := p.Eval(`id=>{`+webDOMHelpers+postDOM+`if(!postCaption(id))throw Error('Cannot bind post caption');return [id,richText(postCaption(id))];}`, id)
	if err != nil {
		return "", err
	}
	var fields []string
	if err := r.Value.Unmarshal(&fields); err != nil {
		return "", err
	}
	return webDigest(fields), nil
}

func (s *WebService) webIdentity(ctx context.Context, p *rod.Page) (string, string, error) {
	if err := checkWebPage(p); err != nil {
		return "", "", err
	}
	state, err := webSnapshot(p)
	if err != nil {
		return "", "", err
	}
	items, err := s.browser.Cookies(ctx)
	if err != nil {
		return "", "", err
	}
	fingerprint := sessionFingerprint(items)
	if !state.Account || fingerprint == "" {
		return "", "", problem("login_required", "未能确认当前网页版登录账号，未操作", 401)
	}
	r, err := p.Eval(`()=>{const a=document.querySelector('#comment-input-container a[href*="/user/"]');return a?.href||'';}`)
	if err != nil {
		return "", "", err
	}
	return fingerprint, cleanPublicURL(r.Value.Str()), nil
}

func (s *WebService) prepareInteraction(ctx context.Context, kind string, r *InteractionRequest) (*InteractionResult, error) {
	// Detach all caller-owned pointers/slices before a two-phase operation.
	data, _ := json.Marshal(r)
	var request InteractionRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, err
	}
	r = &request
	var media *stagedMedia
	var err error
	if r.ImagePath != "" {
		media, err = stageMedia(ctx, s.mediaRoot, s.dataDir, "image", &PublishRequest{ImagePaths: []string{r.ImagePath}})
		if err != nil {
			return nil, err
		}
	}
	keep := false
	defer func() {
		if !keep && media != nil {
			_ = os.RemoveAll(media.Dir)
		}
	}()
	// A previous uploaded attachment must not leak into a new preview.
	s.discardInteraction()
	p, postID, err := s.openPost(ctx, r.Post)
	if err != nil {
		return nil, err
	}
	defer func() {
		if keep {
			return
		}
		// A failed preparation can leave text, a picker, or optimistic UI state
		// in the retained detail page. Never reuse that page for another action.
		browser.ClosePage(s.detailPage)
		s.detailPage = nil
		s.detailID = ""
	}()
	fingerprint, actor, err := s.webIdentity(ctx, p)
	if err != nil {
		return nil, err
	}
	postProof, err := postContentProof(p, postID)
	if err != nil {
		return nil, err
	}
	if err := dismissCommentPickers(p); err != nil {
		return nil, err
	}
	id, err := webNonce()
	if err != nil {
		return nil, err
	}
	expires := time.Now().Add(30 * time.Minute)
	result := &InteractionResult{Stage: "ready", ActionID: id, Kind: kind, PostID: postID, PostURL: r.Post, Text: r.Text, Liked: r.Liked, Favorited: r.Favorited, Disliked: r.Disliked, Emojis: append([]string{}, r.Emojis...), ExpiresAt: &expires, Message: "尚未执行。请核对账号、作品、目标评论和内容，再只传 action_id 与 confirm:true 执行一次；30 分钟内有效，新预览会替换旧预览。"}
	info, err := p.Info()
	if err != nil {
		return nil, err
	}
	result.PostURL = info.URL
	d := &interactionDraft{id: id, kind: kind, postID: postID, fingerprint: fingerprint, postProof: postProof, actorURL: actor, request: *r, page: s.detailPage, media: media, result: result}
	if r.CommentRef != "" {
		target, err := s.commentTarget(postID, r.CommentRef)
		if err != nil {
			return nil, err
		}
		if _, err := findComment(p, postID, target); err != nil {
			return nil, err
		}
		d.target = &target
		c := target.Comment
		result.Target = &c
	}
	if kind == "comment" || kind == "reply" {
		if actor == "" {
			return nil, problem("account_unknown", "无法识别评论编辑器所属账号，未发送", 409)
		}
		var box *rod.Element
		if d.target != nil {
			box, err = replyEditor(ctx, p, postID, *d.target)
		} else {
			box, err = commentEditor(ctx, p, postID)
		}
		if err != nil {
			return nil, err
		}
		if err := replaceCommentText(box, ""); err != nil {
			return nil, err
		}
		initial, err := readEditor(p)
		if err != nil {
			return nil, err
		}
		if initial.Text != "" || len(initial.Mentions) > 0 {
			return nil, problem("editor_not_empty", "评论编辑器未能清空，未发送", 409)
		}
		if len(initial.Files) > 0 || len(initial.Images) > 0 {
			return nil, problem("existing_attachment", "编辑器仍有旧图片，请先在专用页面移除或重新打开作品", 409)
		}
		if d.target != nil {
			if err := verifyReplyPlaceholder(initial, *d.target); err != nil {
				return nil, err
			}
		} else if initial.OwnerKey != "" {
			return nil, problem("reply_target_unknown", "顶层评论意外指向其他评论", 409)
		}
		if err := appendCommentText(box, r.Text); err != nil {
			return nil, err
		}
		if media != nil {
			if err := attachCommentImage(ctx, p, media); err != nil {
				return nil, err
			}
			result.ImageName = media.Names[0]
			result.Message += " 本次准备已将图片上传给抖音，但尚未发送评论。"
		}
		for _, ref := range r.Mentions {
			candidate, err := s.selectMention(ctx, p, postID, box, ref)
			if err != nil {
				return nil, err
			}
			result.Mentions = append(result.Mentions, candidate)
		}
		for _, ref := range r.Emojis {
			if err := s.selectEmoji(ctx, p, postID, ref); err != nil {
				return nil, err
			}
		}
		if err := clickOnce(box); err != nil {
			return nil, err
		} // dismiss picker, never submit
		state, err := readEditor(p)
		if err != nil {
			return nil, err
		}
		if d.target != nil {
			if err := verifyReplyPlaceholder(state, *d.target); err != nil {
				return nil, err
			}
		}
		if _, err := commentSendButton(p); err != nil {
			return nil, err
		}
		d.proof = webDigest(state)
		result.Text = state.Text
	} else {
		if _, err := s.reactionState(p, d, false); err != nil {
			return nil, err
		}
	}
	if err := s.login.save(ctx); err != nil {
		return nil, err
	}
	s.active = d
	keep = true
	return result, nil
}

func attachCommentImage(ctx context.Context, p *rod.Page, media *stagedMedia) error {
	items, err := p.ElementsByJS(rod.Eval(`()=>[...document.querySelectorAll('` + composerSelector + ` input[type="file"]')].filter(e=>!e.disabled&&/png|image/.test(e.accept))`))
	el, err := exactlyOne(items, err, "评论图片上传控件")
	if err != nil {
		return err
	}
	if err := el.SetFiles(media.Paths); err != nil {
		return err
	}
	limited, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	err = poll(limited, 250*time.Millisecond, func() (bool, error) {
		if err := checkWebPage(p.Context(limited)); err != nil {
			return false, err
		}
		state, err := readEditor(p.Context(limited))
		if err != nil {
			return false, err
		}
		// The live site clears input.files once it accepts the upload. Require
		// one decoded attachment preview, not a stale browser FileList. Inline
		// emoji and picker avatars cannot satisfy the attachment check.
		if len(state.Files) > 1 || len(state.Images) == 0 {
			return false, nil
		}
		ready, err := p.Context(limited).Eval(`()=>{const root=document.querySelector('` + composerSelector + `');if(!root)return false;const images=[...root.querySelectorAll('img')].filter(e=>!e.closest('a,[contenteditable],.emoji-card-outer-container,.atBox-inner-container'));return images.length===1&&images[0].complete&&images[0].naturalWidth>0;}`)
		if err != nil {
			return false, err
		}
		if !ready.Value.Bool() {
			return false, nil
		}
		_, err = commentSendButton(p.Context(limited))
		return err == nil, nil
	})
	return wrapTimeout(err, "图片预览/发送状态未就绪，未发送；请检查文件格式、账号权限或页面提示")
}

func desiredReaction(d *interactionDraft) bool {
	if d.kind == "favorite" {
		return *d.request.Favorited
	}
	if d.kind == "comment_dislike" {
		return *d.request.Disliked
	}
	return *d.request.Liked
}

func (s *WebService) reactionState(p *rod.Page, d *interactionDraft, after bool) (*bool, error) {
	if d.target == nil {
		return postReactionState(p, d.postID, d.kind)
	}
	t := *d.target
	if after {
		// Disliking deliberately replaces the body with “该评论被折叠”. Only
		// after our one click, verify by the same exact DOM ID + author + parent.
		rows, err := readCommentRecords(p, d.postID)
		if err != nil {
			return nil, err
		}
		found := 0
		for _, r := range rows {
			if r.DOMKey == t.DOMKey && r.AuthorURL == t.Comment.AuthorURL && r.ParentKey == t.ParentDOMKey {
				t.Signature = r.Signature
				found++
			}
		}
		if found != 1 {
			return nil, problem("target_changed", "操作后的目标评论不唯一或已消失", 409)
		}
	}
	return commentReactionState(p, d.postID, t, d.kind)
}

func (s *WebService) confirmInteraction(ctx context.Context, kind, id string) (*InteractionResult, error) {
	if prior, err := s.interactionPrior(kind, id); prior != nil || err != nil {
		return prior, err
	}
	d := s.active
	if d == nil || d.id != id || d.kind != kind {
		return nil, problem("action_not_found", "预览已失效、类型不符或服务已重启；未执行", 409)
	}
	if d.attempted {
		return d.result, problem("interaction_unknown", "此操作已尝试过一次，未再次执行", 409)
	}
	if d.result.ExpiresAt == nil || time.Now().After(*d.result.ExpiresAt) {
		s.closeInteraction()
		return nil, problem("action_expired", "预览已完成或超过 30 分钟", 409)
	}
	p := d.page.Context(ctx)
	fingerprint, actor, err := s.webIdentity(ctx, p)
	if err != nil {
		return nil, err
	}
	if fingerprint != d.fingerprint || actor != d.actorURL {
		return nil, problem("account_changed", "账号或登录会话已变化，请重新准备", 409)
	}
	proof, err := postContentProof(p, d.postID)
	if err != nil {
		return nil, err
	}
	if proof != d.postProof {
		return nil, problem("post_changed", "目标作品文案发生变化，请重新确认", 409)
	}
	var button *rod.Element
	var before []commentRecord
	isComment := kind == "comment" || kind == "reply"
	if isComment {
		if d.target != nil {
			if _, err := findComment(p, d.postID, *d.target); err != nil {
				return nil, err
			}
		}
		state, err := readEditor(p)
		if err != nil {
			return nil, err
		}
		if webDigest(state) != d.proof {
			return nil, problem("editor_changed", "评论正文、提及、图片或回复对象与准备时不一致；未发送", 409)
		}
		if d.target != nil {
			if err := verifyReplyPlaceholder(state, *d.target); err != nil {
				return nil, err
			}
		} else if state.OwnerKey != "" {
			return nil, problem("editor_changed", "顶层评论被改为回复", 409)
		}
		before, err = readCommentRecords(p, d.postID)
		if err != nil {
			return nil, err
		}
		button, err = commentSendButton(p)
		if err != nil {
			return nil, err
		}
	} else {
		if d.target == nil && s.pendingReactions[d.kind] {
			return nil, problem("interaction_unknown", "当前页面仍有未确认的反应状态；不能把临时变色当作已完成，也不会再次点击", 409)
		}
		state, err := s.reactionState(p, d, false)
		if err != nil {
			return nil, err
		}
		if *state == desiredReaction(d) {
			d.result.Verification = "page_state"
			return s.completeInteraction(d, "unchanged", "当前已经是所需状态，没有重复点击")
		}
		if d.target != nil {
			button, err = commentReactionButton(p, d.postID, *d.target, kind)
		} else {
			button, err = postReactionButton(p, d.postID, kind)
		}
		if err != nil {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ack, err := observeActionAck(ctx, p, d)
	if err != nil {
		return nil, err
	}
	if ack != nil {
		defer ack.stop()
	}
	d.result.Stage = "submitting"
	if err := s.interactionJournal(d.result); err != nil {
		d.result.Stage = "ready"
		return nil, err
	}
	d.attempted = true
	if !isComment && d.target == nil {
		if s.pendingReactions == nil {
			s.pendingReactions = map[string]bool{}
		}
		s.pendingReactions[d.kind] = true
	}
	ack.arm()
	if err := clickOnce(button); err != nil {
		return s.uncertainInteraction(d, err)
	}
	limited, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	err = s.verifyInteractionResult(limited, p.Context(limited), d, before, ack)
	if err != nil {
		return s.uncertainInteraction(d, err)
	}
	if !isComment && d.target == nil {
		delete(s.pendingReactions, d.kind)
	}
	message := "当前页面已确认操作结果；同一 action_id 再次确认只返回回执，不会重复操作。评论可见性仍由平台审核决定。"
	if d.result.Verification == "platform_response" {
		message = "本次提交的业务成功响应已确认（不只检查 HTTP 200），未重新打开作品。不会重复操作；评论可见性仍由平台审核决定。"
	}
	return s.completeInteraction(d, "completed", message)
}

func (s *WebService) completeInteraction(d *interactionDraft, stage, message string) (*InteractionResult, error) {
	d.attempted = true // Also close an unchanged operation if persisting its receipt fails.
	d.result.Stage = stage
	d.result.Success = true
	d.result.ExpiresAt = nil
	d.result.Message = message
	if err := s.interactionJournal(d.result); err != nil {
		return d.result, problem("receipt_failed", "页面状态已确认，但回执保存失败；不要另建操作盲目重发", 500)
	}
	return d.result, nil
}

func commentSentOnPage(p *rod.Page, d *interactionDraft, before []commentRecord) (bool, error) {
	seen := map[string]bool{}
	for _, r := range before {
		seen[r.DOMKey] = true
	}
	rows, err := readCommentRecords(p, d.postID)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if seen[row.DOMKey] || row.AuthorURL != d.actorURL || normalizedText(row.Text) != normalizedText(d.result.Text) {
			continue
		}
		if d.target == nil && row.ParentKey != "" {
			continue
		}
		if d.target != nil && row.ParentKey != d.target.DOMKey && (d.target.ParentDOMKey == "" || row.ParentKey != d.target.ParentDOMKey) {
			continue
		}
		if d.media != nil && len(row.Images) == 0 {
			continue
		}
		return true, nil
	}
	// Generic or old success toasts, a cleared editor, and changed counts
	// are not evidence. If moderation hides the new row, return unknown.
	return false, nil
}

func (s *WebService) closeInteraction() {
	if s.active != nil && s.active.media != nil {
		_ = os.RemoveAll(s.active.media.Dir)
	}
	s.active = nil
}

func (s *WebService) discardInteraction() {
	if s.active != nil && s.active.media != nil {
		browser.ClosePage(s.detailPage)
		s.detailPage = nil
		s.detailID = ""
	}
	s.closeInteraction()
}
