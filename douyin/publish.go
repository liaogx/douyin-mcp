package douyin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/liaogx/douyin-mcp/browser"
	"github.com/liaogx/douyin-mcp/internal/securefile"
	"os"
	"path/filepath"
	"time"
)

type draft struct {
	id, kind, fingerprint, proof string
	request                      PublishRequest
	page                         *rod.Page
	media                        *stagedMedia
	result                       *PublishResult
	attempted                    bool
}

type PublishService struct {
	browser                       Browser
	login                         *LoginService
	mediaRoot, dataDir, uploadURL string
	active                        *draft
}

func NewPublishService(br Browser, login *LoginService, mediaRoot, dataDir string) *PublishService {
	return &PublishService{browser: br, login: login, mediaRoot: mediaRoot, dataDir: dataDir, uploadURL: CreatorUpload}
}

func (s *PublishService) Close() {
	if s.active != nil {
		browser.ClosePage(s.active.page)
		if s.active.media != nil {
			_ = os.RemoveAll(s.active.media.Dir)
		}
		s.active = nil
	}
}

func (s *PublishService) PublishVideo(ctx context.Context, req *PublishRequest) (*PublishResult, error) {
	return s.publish(ctx, "video", req)
}
func (s *PublishService) PublishImageText(ctx context.Context, req *PublishRequest) (*PublishResult, error) {
	return s.publish(ctx, "image", req)
}

func (s *PublishService) publish(ctx context.Context, kind string, req *PublishRequest) (*PublishResult, error) {
	if err := validateRequest(kind, req); err != nil {
		return nil, err
	}
	if req.Confirm {
		return s.confirm(ctx, kind, req.DraftID)
	}
	return s.prepare(ctx, kind, req)
}

func (s *PublishService) prepare(ctx context.Context, kind string, req *PublishRequest) (*PublishResult, error) {
	// Validate and seal the local files before navigating/uploading anything.
	media, err := stageMedia(ctx, s.mediaRoot, s.dataDir, kind, req)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(media.Dir)
		}
	}()
	login, err := s.login.CheckLoginStatus(ctx)
	if err != nil {
		return nil, err
	}
	if !login.Success {
		return nil, problem("login_required", "请先扫码登录并检查登录状态", 401)
	}
	items, err := s.browser.Cookies(ctx)
	if err != nil {
		return nil, err
	}
	fingerprint := sessionFingerprint(items)
	s.Close() // A new preparation explicitly replaces the previous local preview.
	page, err := s.browser.NewPage(ctx, s.uploadURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !keep {
			browser.ClosePage(page)
		}
	}()
	p := page.Context(ctx)
	if err := openUpload(ctx, p, kind, media.Paths); err != nil {
		return nil, err
	}
	if err := fillMetadata(ctx, p, req); err != nil {
		return nil, err
	}
	if err := waitReady(ctx, p); err != nil {
		return nil, wrapTimeout(err, "素材仍未就绪，请在专用浏览器确认上传状态")
	}
	proof, err := mediaProof(p)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	id := hex.EncodeToString(nonce)
	expires := time.Now().Add(30 * time.Minute)
	result := &PublishResult{Stage: "ready", DraftID: id, Title: req.Title, Description: req.Description, Files: media.Names, Nickname: login.Nickname, ExpiresAt: &expires, Message: "素材已上传并填好文案，但尚未发布，也未承诺保存为平台草稿。请核对专用浏览器中的账号、内容和可见范围，再用此 draft_id 与 confirm:true 确认；30 分钟内有效，新预览会替换旧预览"}
	s.active = &draft{id: id, kind: kind, fingerprint: fingerprint, proof: proof, request: *req, page: page, media: media, result: result}
	keep = true
	return result, nil
}

func (s *PublishService) journal(id string, result *PublishResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return securefile.Write(filepath.Join(s.dataDir, "receipts", id+".json"), data)
}

func (s *PublishService) prior(id string) (*PublishResult, error) {
	data, err := securefile.Read(filepath.Join(s.dataDir, "receipts", id+".json"), 1<<20)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var result PublishResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result.DraftID != id {
		return nil, errors.New("发布回执不一致，未再次提交")
	}
	if result.Stage != "submitted" {
		result.Stage = "unknown"
		result.Success = false
		result.Message = "该预览已有一次提交记录，但结果未确认；请到创作者中心核对，程序不会再次点击发布"
		return &result, problem("publish_unknown", result.Message, 409)
	}
	return &result, nil
}

func (s *PublishService) confirm(ctx context.Context, kind, id string) (*PublishResult, error) {
	if prior, err := s.prior(id); prior != nil || err != nil {
		return prior, err
	}
	d := s.active
	if d == nil || d.id != id || d.kind != kind {
		return nil, problem("draft_not_found", "预览已失效、类型不匹配或服务已重启，请重新准备；未发布", 409)
	}
	if d.attempted {
		return d.result, problem("publish_unknown", "该预览已尝试提交，未再次点击发布", 409)
	}
	if time.Now().After(*d.result.ExpiresAt) {
		s.Close()
		return nil, problem("draft_expired", "预览超过 30 分钟，请重新准备", 409)
	}
	items, err := s.browser.Cookies(ctx)
	if err != nil {
		return nil, err
	}
	if sessionFingerprint(items) == "" || sessionFingerprint(items) != d.fingerprint {
		return nil, problem("account_changed", "登录账号或会话发生变化，请重新准备并确认", 409)
	}
	p := d.page.Context(ctx)
	if err := verifyMetadata(p, &d.request); err != nil {
		return nil, err
	}
	proof, err := mediaProof(p)
	if err != nil {
		return nil, err
	}
	if proof != d.proof {
		return nil, problem("media_changed", "页面素材与预览时不一致，请重新准备", 409)
	}
	readyCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := waitReady(readyCtx, p.Context(readyCtx)); err != nil {
		return nil, wrapTimeout(err, "发布按钮不可用，未提交")
	}
	if already, err := publicationConfirmed(p); err != nil {
		return nil, err
	} else if already {
		return nil, problem("unexpected_page", "页面已显示历史成功状态，未重复发布", 409)
	}
	button, err := publishButton(p)
	if err != nil {
		return nil, err
	}
	if button == nil {
		return nil, problem("publish_unavailable", "发布按钮不可用", 409)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	d.result.Stage = "submitting"
	// Durable write BEFORE the one and only click. On interruption we fail closed.
	if err := s.journal(id, d.result); err != nil {
		d.result.Stage = "ready"
		return nil, err
	}
	d.attempted = true
	if err := button.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return s.uncertain(d, err)
	}
	verifyCtx, verifyCancel := context.WithTimeout(ctx, 45*time.Second)
	defer verifyCancel()
	if err := verifyPublished(verifyCtx, p.Context(verifyCtx)); err != nil {
		return s.uncertain(d, err)
	}
	now := time.Now()
	d.result.Success = true
	d.result.Stage = "submitted"
	d.result.PublishAt = &now
	d.result.ExpiresAt = nil
	d.result.Message = "抖音页面已确认提交成功；作品是否公开仍取决于平台审核。重复确认此 draft_id 只返回回执，不会重复发布"
	if err := s.journal(id, d.result); err != nil {
		return d.result, problem("receipt_failed", "页面已确认提交，但本地回执保存失败；不要重试发布，请核对创作者中心", 500)
	}
	return d.result, nil
}

func (s *PublishService) uncertain(d *draft, cause error) (*PublishResult, error) {
	d.result.Success = false
	d.result.Stage = "unknown"
	d.result.Message = "已尝试点击发布，但结果未确认。请到创作者中心核对，程序不会自动重试"
	_ = s.journal(d.id, d.result)
	return d.result, problem("publish_unknown", d.result.Message+": "+cause.Error(), 409)
}
