package douyin

import (
	"context"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"time"
)

const CreatorHome = "https://creator.douyin.com/creator-micro/home"
const CreatorUpload = "https://creator.douyin.com/creator-micro/content/upload"

type Browser interface {
	NewPage(context.Context, string) (*rod.Page, error)
	Cookies(context.Context) ([]*proto.NetworkCookie, error)
	Restore(context.Context, []*proto.NetworkCookieParam) error
	Reset(context.Context) error
}

type LoginPhase string

const (
	PhaseQRCode         LoginPhase = "qrcode"
	PhaseLoggedIn       LoginPhase = "logged_in"
	PhaseNeedsAttention LoginPhase = "needs_attention"
	PhaseUnknown        LoginPhase = "unknown"
)

type LoginResult struct {
	Phase    LoginPhase `json:"phase"`
	Success  bool       `json:"success"`
	Nickname string     `json:"nickname,omitempty"`
	Message  string     `json:"message"`
	QRImage  string     `json:"qr_image,omitempty"`
	QRPNG    []byte     `json:"-"`
}

// A confirmation only accepts draft_id + confirm, preventing changed metadata
// from being silently substituted for the content the user has reviewed.
type PublishRequest struct {
	VideoPath   string   `json:"video_path,omitempty" jsonschema:"Video path under the configured media root; required when preparing a video"`
	ImagePaths  []string `json:"image_paths,omitempty" jsonschema:"Ordered image paths under the media root; required when preparing image-text"`
	Title       string   `json:"title,omitempty" jsonschema:"Title, 1-30 Unicode characters"`
	Description string   `json:"description,omitempty" jsonschema:"Body text, up to 1000 Unicode characters"`
	DraftID     string   `json:"draft_id,omitempty" jsonschema:"The exact draft ID returned by preparation"`
	Confirm     bool     `json:"confirm,omitempty" jsonschema:"False prepares a preview only. True submits the reviewed draft exactly once"`
}

type PublishResult struct {
	Success     bool       `json:"success"`
	Stage       string     `json:"stage"`
	DraftID     string     `json:"draft_id,omitempty"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Files       []string   `json:"files,omitempty"`
	Nickname    string     `json:"nickname,omitempty"`
	Message     string     `json:"message"`
	AwemeID     string     `json:"aweme_id,omitempty"`
	URL         string     `json:"url,omitempty"`
	PublishAt   *time.Time `json:"publish_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type Error struct {
	Code, Message string
	Status        int
}

func (e *Error) Error() string                       { return e.Message }
func problem(code, message string, status int) error { return &Error{code, message, status} }
