package douyin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const maxAckBody = 128 << 10

// This is a conservative compatibility adapter, not an API client. Only a
// POST emitted by the existing page after our authorized click is observed.
// Unknown endpoints/shapes fail closed. No signing, fetch, replay, response
// interception, request header logging, or persistent network trace is used.
func ackPath(d *interactionDraft) string {
	if d.target != nil {
		return ""
	}
	switch d.kind {
	case "comment":
		if d.media == nil && len(d.request.Mentions) == 0 && len(d.request.Emojis) == 0 {
			return "/aweme/v1/web/comment/publish/"
		}
	case "like":
		return "/aweme/v1/web/commit/item/digg/"
	case "favorite":
		return "/aweme/v1/web/aweme/collect/"
	}
	return ""
}

func (s *WebService) verifyInteractionResult(ctx context.Context, p *rod.Page, d *interactionDraft, before []commentRecord, ack *actionAck) error {
	started := time.Now()
	var stableSince time.Time
	isComment := d.kind == "comment" || d.kind == "reply"
	return poll(ctx, 150*time.Millisecond, func() (bool, error) {
		if err := verifyPost(p, d.postID); err != nil {
			return false, err
		}
		observed := ack.snapshot()
		if observed.Matched {
			if !observed.Done {
				return false, nil
			}
			if observed.Err != nil {
				return false, observed.Err
			}
			if err := s.verifyAckContext(ctx, p, d); err != nil {
				return false, err
			}
			if !isComment {
				state, err := s.reactionState(p, d, true)
				if err != nil || state == nil || *state != desiredReaction(d) {
					return false, err
				}
			}
			d.result.Verification, d.result.CommentID = "platform_response", observed.CommentID
			return true, nil
		}
		if isComment {
			// Allow the current page's request to arrive before using DOM-only
			// evidence. No reload, new tab, or extra network query is issued.
			if ack != nil && time.Since(started) < time.Second {
				return false, nil
			}
			found, err := commentSentOnPage(p, d, before)
			if found && err == nil {
				d.result.Verification = "visible_comment"
			}
			return found, err
		}
		if d.target == nil {
			// Post reactions are optimistic on the live site. A colored icon is
			// NOT a server acknowledgement, even if it stays colored for seconds.
			return false, nil
		}
		state, err := s.reactionState(p, d, true)
		if err != nil {
			return false, err
		}
		if state == nil || *state != desiredReaction(d) {
			stableSince = time.Time{}
			return false, nil
		}
		if stableSince.IsZero() {
			stableSince = time.Now()
		}
		if time.Since(stableSince) < 500*time.Millisecond {
			return false, nil
		}
		d.result.Verification = "page_state"
		return true, nil
	})
}

func matchesAckRequest(d *interactionDraft, req *proto.NetworkRequest) bool {
	if req == nil || req.Method != "POST" || len(req.URL) > 32<<10 || len(req.PostData) > 32<<10 {
		return false
	}
	u, err := url.Parse(req.URL)
	if err != nil || u.Scheme != "https" || u.Host != "www.douyin.com" || u.User != nil || u.Fragment != "" || ackPath(d) == "" || u.Path != ackPath(d) {
		return false
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return false
	}
	body, err := url.ParseQuery(req.PostData)
	if err != nil {
		return false
	}
	// Ignore unrelated signing/telemetry parameters; never retain them. Reject
	// duplicate binding keys rather than guessing which one the server uses.
	value := func(key string) (string, bool) {
		values := append(q[key], body[key]...)
		if len(values) != 1 {
			return "", false
		}
		return values[0], true
	}
	id, ok := value("aweme_id")
	if !ok || id != d.postID {
		return false
	}
	if d.kind == "comment" {
		text, ok := value("text")
		if !ok || text != d.result.Text {
			return false
		}
		for _, key := range []string{"reply_id", "reply_to_reply_id"} {
			values := append(q[key], body[key]...)
			if len(values) > 1 || len(values) == 1 && values[0] != "" && values[0] != "0" {
				return false
			}
		}
		return true
	}
	typ, ok := value("type")
	want := "0"
	if desiredReaction(d) {
		want = "1"
	}
	return ok && typ == want
}

func parseActionAck(d *interactionDraft, status int, body []byte) (string, error) {
	if status != 200 || len(body) == 0 || len(body) > maxAckBody {
		return "", errors.New("提交响应不完整或 HTTP 状态异常；不能确认业务成功")
	}
	var response struct {
		Status  *int `json:"status_code"`
		Comment *struct {
			ID        string `json:"cid"`
			PostID    string `json:"aweme_id"`
			Text      string `json:"text"`
			ReplyID   string `json:"reply_id"`
			ReplyToID string `json:"reply_to_reply_id"`
			User      struct {
				SecUID string `json:"sec_uid"`
			} `json:"user"`
		} `json:"comment"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.Status == nil {
		return "", errors.New("提交响应格式未识别；HTTP 200 不能单独证明成功")
	}
	if *response.Status != 0 {
		return "", errors.New("平台返回业务失败或验证要求；未将 HTTP 200 当作成功")
	}
	if d.kind != "comment" {
		return "", nil
	}
	c := response.Comment
	if c == nil || !postIDPattern.MatchString(c.ID) || c.PostID != d.postID || c.Text != d.result.Text || c.User.SecUID == "" ||
		"https://www.douyin.com/user/"+c.User.SecUID != d.actorURL ||
		(c.ReplyID != "" && c.ReplyID != "0") || (c.ReplyToID != "" && c.ReplyToID != "0") {
		return "", errors.New("评论响应未能绑定新评论 ID、作品、正文及当前账号；结果未确认")
	}
	return c.ID, nil
}

type actionAckState struct {
	Matched, Done bool
	CommentID     string
	Err           error
}

type actionAck struct {
	mu    sync.Mutex
	state actionAckState
	armed time.Time
	stop  func()
}

func (a *actionAck) arm() {
	if a != nil {
		a.mu.Lock()
		a.armed = time.Now()
		a.mu.Unlock()
	}
}

func (a *actionAck) snapshot() actionAckState {
	if a == nil {
		return actionAckState{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state
}

func observeActionAck(ctx context.Context, p *rod.Page, d *interactionDraft) (*actionAck, error) {
	if ackPath(d) == "" {
		return nil, nil
	}
	ctx, cancel := context.WithCancel(ctx)
	p = p.Context(ctx)
	var previous proto.NetworkEnable
	hadNetwork := p.LoadState(&previous)
	total, resource, post := 512<<10, maxAckBody, 32<<10
	if err := (proto.NetworkEnable{MaxTotalBufferSize: &total, MaxResourceBufferSize: &resource, MaxPostDataSize: &post}).Call(p); err != nil {
		cancel()
		return nil, errors.New("无法监听本次提交响应；未执行操作")
	}
	a := &actionAck{}
	var requestID proto.NetworkRequestID
	status := 0
	finishError := func(message string) {
		a.state.Done = true
		a.state.Err = errors.New(message)
	}
	wait := p.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if e.RequestID == requestID && e.RedirectResponse != nil {
			finishError("提交发生重定向；结果未确认")
			return
		}
		if a.armed.IsZero() || float64(e.WallTime) < float64(a.armed.UnixNano())/1e9 || e.FrameID != p.FrameID ||
			!samePostURL(e.DocumentURL, d.postID) || !matchesAckRequest(d, e.Request) {
			return
		}
		if a.state.Matched {
			finishError("观察到多个匹配的提交请求；停止自动判断，不会重发")
			return
		}
		requestID, a.state.Matched = e.RequestID, true
	}, func(e *proto.NetworkResponseReceived) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !a.state.Matched || e.RequestID != requestID || a.state.Done {
			return
		}
		r := e.Response
		if r == nil || r.FromDiskCache || r.FromServiceWorker || r.FromPrefetchCache {
			finishError("响应来源不是本次服务器提交；结果未确认")
			return
		}
		u, err := url.Parse(r.URL)
		if err != nil || u.Scheme != "https" || u.Host != "www.douyin.com" || u.User != nil || u.Path != ackPath(d) {
			finishError("提交响应来源变化；结果未确认")
			return
		}
		status = r.Status
	}, func(e *proto.NetworkLoadingFailed) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.state.Matched && e.RequestID == requestID {
			finishError("本次提交连接失败或中断；结果未确认，不自动重发")
		}
	}, func(e *proto.NetworkLoadingFinished) {
		a.mu.Lock()
		defer a.mu.Unlock()
		if !a.state.Matched || e.RequestID != requestID || a.state.Done {
			return
		}
		if e.EncodedDataLength > maxAckBody {
			finishError("提交响应超过读取上限；结果未确认")
			return
		}
		limited, release := context.WithTimeout(ctx, time.Second)
		defer release()
		body, err := (proto.NetworkGetResponseBody{RequestID: requestID}).Call(p.Context(limited))
		if err != nil || len(body.Body) > maxAckBody*2 {
			finishError("无法读取本次提交响应；结果未确认")
			return
		}
		data := []byte(body.Body)
		if body.Base64Encoded {
			data, err = base64.StdEncoding.DecodeString(body.Body)
		}
		if err != nil {
			finishError("提交响应编码未识别；结果未确认")
			return
		}
		a.state.CommentID, a.state.Err = parseActionAck(d, status, data)
		a.state.Done = true
	})
	done := make(chan struct{})
	go func() { defer close(done); wait() }()
	a.stop = sync.OnceFunc(func() {
		cancel()
		<-done
		cleanup, release := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer release()
		if hadNetwork {
			_ = previous.Call(p.Context(cleanup))
		} else {
			_ = (proto.NetworkDisable{}).Call(p.Context(cleanup))
		}
	})
	return a, nil
}

// Passive response observation never grants permission to interact. This final
// check binds a successful receipt to the same logged-in actor and visible post.
func (s *WebService) verifyAckContext(ctx context.Context, p *rod.Page, d *interactionDraft) error {
	fingerprint, actor, err := s.webIdentity(ctx, p)
	if err != nil {
		return err
	}
	if fingerprint != d.fingerprint || actor != d.actorURL {
		return problem("account_changed", "提交期间账号发生变化；结果需人工核对", 409)
	}
	proof, err := postContentProof(p, d.postID)
	if err != nil {
		return err
	}
	if proof != d.postProof {
		return problem("post_changed", "提交期间作品发生变化；结果需人工核对", 409)
	}
	return nil
}
