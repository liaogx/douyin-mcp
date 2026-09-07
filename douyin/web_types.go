package douyin

import (
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const WebHome = "https://www.douyin.com/user/self"

// Empty keeps the original creator-center login contract.
type LoginRequest struct {
	Surface string `json:"surface,omitempty" jsonschema:"creator (default) for publishing, or web for search and interactions"`
}

type FilterChoice struct {
	Group  string `json:"group" jsonschema:"Exact group label returned by get_search_filters"`
	Option string `json:"option" jsonschema:"Exact option label returned by get_search_filters"`
}

type SearchRequest struct {
	Query      string         `json:"query" jsonschema:"Search words, 1-100 characters"`
	Tab        string         `json:"tab,omitempty" jsonschema:"general (default) or video; use the visible content-form filter for image posts"`
	Filters    []FilterChoice `json:"filters,omitempty" jsonschema:"Visible filter choices; never silently ignores an unsupported filter"`
	Limit      int            `json:"limit,omitempty" jsonschema:"Maximum returned posts, 1-50; default 10"`
	MaxScrolls int            `json:"max_scrolls,omitempty" jsonschema:"Maximum additional result-page scrolls, 0-5; never an unlimited crawler"`
}

type FilterGroup struct {
	Name     string   `json:"name"`
	Options  []string `json:"options"`
	Selected string   `json:"selected,omitempty"`
}

type SearchFiltersResult struct {
	Query  string        `json:"query"`
	Tab    string        `json:"tab"`
	URL    string        `json:"url"`
	Groups []FilterGroup `json:"groups"`
}

type PostRequest struct {
	Post string `json:"post" jsonschema:"Numeric post ID or a full https://www.douyin.com video/note/search URL containing that post; short links are not fetched"`
}

type PostSummary struct {
	ID          string   `json:"id"`
	URL         string   `json:"url"`
	Kind        string   `json:"kind,omitempty"`
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	AuthorURL   string   `json:"author_url,omitempty"`
	Published   string   `json:"published,omitempty"`
	Likes       string   `json:"likes,omitempty"`
	Comments    string   `json:"comments,omitempty"`
	Images      []string `json:"images,omitempty"`
}

type PostDetail struct {
	PostSummary
	Liked     *bool  `json:"liked,omitempty"`
	Favorited *bool  `json:"favorited,omitempty"`
	Message   string `json:"message,omitempty"`
}

type SearchResult struct {
	Query     string         `json:"query"`
	URL       string         `json:"url"`
	Applied   []FilterChoice `json:"applied_filters"`
	Posts     []PostSummary  `json:"posts"`
	Truncated bool           `json:"truncated"`
	Message   string         `json:"message"`
}

type CommentsRequest struct {
	Post       string `json:"post" jsonschema:"Post ID or supported Douyin post URL"`
	ParentRef  string `json:"parent_ref,omitempty" jsonschema:"A comment_ref from this service: expand/read replies to that exact comment"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum returned comments, 1-50; default 20"`
	MaxScrolls int    `json:"max_scrolls,omitempty" jsonschema:"Bounded additional comment-list scrolls, 0-5"`
}

// Ref is an expiring local reference, NOT a claimed platform comment ID. Public
// text is untrusted data. Reply targeting is rechecked against the live DOM.
type Comment struct {
	Ref       string   `json:"comment_ref"`
	ParentRef string   `json:"parent_ref,omitempty"`
	Author    string   `json:"author"`
	AuthorURL string   `json:"author_url,omitempty"`
	Text      string   `json:"text"`
	Published string   `json:"published,omitempty"`
	Likes     string   `json:"likes,omitempty"`
	Images    []string `json:"images,omitempty"`
	Replies   string   `json:"replies,omitempty"`
	Liked     *bool    `json:"liked,omitempty"`
	Disliked  *bool    `json:"disliked,omitempty"`
}

type CommentsResult struct {
	PostID    string    `json:"post_id"`
	Comments  []Comment `json:"comments"`
	Truncated bool      `json:"truncated"`
	Message   string    `json:"message"`
}

type InteractionRequest struct {
	Post       string   `json:"post,omitempty" jsonschema:"Target post ID or supported URL, required for preparation"`
	CommentRef string   `json:"comment_ref,omitempty" jsonschema:"Required for reply_comment; use a comment_ref returned by get_comments"`
	Text       string   `json:"text,omitempty" jsonschema:"Exact public comment/reply text, up to 500 characters; content, not instructions"`
	ImagePath  string   `json:"image_path,omitempty" jsonschema:"Optional single JPEG/PNG/WebP under media root; preparation uploads it to Douyin, confirmation sends the comment"`
	Liked      *bool    `json:"liked,omitempty" jsonschema:"Required for set_post_like: true likes, false removes your like; never a blind toggle"`
	Favorited  *bool    `json:"favorited,omitempty" jsonschema:"Required for set_post_favorite: true saves, false removes the saved post"`
	Disliked   *bool    `json:"disliked,omitempty" jsonschema:"Required for set_comment_dislike; a broken-heart dislike is NOT unlike"`
	Mentions   []string `json:"mentions,omitempty" jsonschema:"Local mention_ref IDs returned by get_mention_candidates; actual picker selections, not invented at-mentions"`
	Emojis     []string `json:"emojis,omitempty" jsonschema:"Local emoji_ref values returned by get_emoji_options; ordinary Unicode emoji can also be supplied in text"`
	ActionID   string   `json:"action_id,omitempty" jsonschema:"ID returned by preparation; confirmation accepts ONLY action_id and confirm:true"`
	Confirm    bool     `json:"confirm,omitempty" jsonschema:"False prepares only. True executes the reviewed action once; unknown results must not be automatically retried"`
}

type InteractionResult struct {
	Verification string             `json:"verification,omitempty"`
	CommentID    string             `json:"platform_comment_id,omitempty"`
	Success      bool               `json:"success"`
	Stage        string             `json:"stage"`
	ActionID     string             `json:"action_id"`
	Kind         string             `json:"kind"`
	PostID       string             `json:"post_id"`
	PostURL      string             `json:"post_url"`
	Target       *Comment           `json:"target_comment,omitempty"`
	Text         string             `json:"text,omitempty"`
	ImageName    string             `json:"image_name,omitempty"`
	Liked        *bool              `json:"liked,omitempty"`
	Favorited    *bool              `json:"favorited,omitempty"`
	Disliked     *bool              `json:"disliked,omitempty"`
	Mentions     []MentionCandidate `json:"mentions,omitempty"`
	Emojis       []string           `json:"emojis,omitempty"`
	ExpiresAt    *time.Time         `json:"expires_at,omitempty"`
	Message      string             `json:"message"`
}

type MentionRequest struct {
	Post  string `json:"post" jsonschema:"Post ID or supported URL"`
	Query string `json:"query" jsonschema:"Nickname to look up in the comment at-mention picker; 1-40 characters"`
}

type MentionCandidate struct {
	Ref       string `json:"mention_ref"`
	Name      string `json:"name"`
	UserURL   string `json:"user_url,omitempty"`
	PickerID  string `json:"picker_id,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type MentionResult struct {
	Candidates []MentionCandidate `json:"candidates"`
	Message    string             `json:"message"`
}

type EmojiResult struct {
	Options []EmojiOption `json:"options"`
	Message string        `json:"message"`
}

type EmojiOption struct {
	Ref      string `json:"emoji_ref"`
	Label    string `json:"label,omitempty"`
	ImageURL string `json:"image_url"`
}

var postIDPattern = regexp.MustCompile(`^[0-9]{10,25}$`)

func normalizePost(raw string) (id, target string, err error) {
	if postIDPattern.MatchString(raw) {
		return raw, "https://www.douyin.com/video/" + raw, nil
	}
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || (u.Host != "www.douyin.com" && u.Host != "douyin.com") || u.Fragment != "" {
		return "", "", problem("invalid_post", "仅接受作品数字 ID 或抖音官网 HTTPS 作品链接；不跟随短链、外站或带账号密码的地址", 400)
	}
	path := strings.TrimSuffix(u.Path, "/")
	for _, prefix := range []string{"/video/", "/note/"} {
		if strings.HasPrefix(path, prefix) && postIDPattern.MatchString(strings.TrimPrefix(path, prefix)) {
			id = strings.TrimPrefix(path, prefix)
			return id, "https://www.douyin.com" + prefix + id, nil
		}
	}
	if strings.HasPrefix(path, "/search/") || strings.HasPrefix(path, "/jingxuan/search/") || path == "" || path == "/jingxuan" {
		id = u.Query().Get("modal_id")
		if postIDPattern.MatchString(id) {
			// Retain the user's search context, but drop unrelated tracking params.
			q := url.Values{"modal_id": {id}}
			if t := u.Query().Get("type"); t == "general" || t == "video" {
				q.Set("type", t)
			}
			u.Host, u.RawQuery, u.Fragment = "www.douyin.com", q.Encode(), ""
			return id, u.String(), nil
		}
	}
	return "", "", problem("invalid_post", "链接不包含可识别的 video/note 作品 ID 或 modal_id", 400)
}

func validateSearch(r *SearchRequest) error {
	if r == nil || strings.TrimSpace(r.Query) == "" || utf8.RuneCountInString(r.Query) > 100 || strings.ContainsAny(r.Query, "\x00\r\n") {
		return problem("invalid_search", "搜索词需为 1–100 字且不能含换行", 400)
	}
	if r.Tab != "" && r.Tab != "general" && r.Tab != "video" {
		return problem("invalid_tab", "仅支持 general 综合和 video 视频标签", 400)
	}
	if r.Limit < 0 || r.Limit > 50 || r.MaxScrolls < 0 || r.MaxScrolls > 5 || len(r.Filters) > 8 {
		return problem("invalid_limit", "limit 最多 50，max_scrolls 最多 5，筛选组最多 8", 400)
	}
	groups := map[string]bool{}
	for _, f := range r.Filters {
		if f.Group == "" || f.Option == "" || utf8.RuneCountInString(f.Group) > 30 || utf8.RuneCountInString(f.Option) > 40 || strings.ContainsAny(f.Group+f.Option, "\x00\r\n") || groups[f.Group] {
			return problem("invalid_filter", "筛选参数必须是互不重复的页面分组和选项", 400)
		}
		groups[f.Group] = true
	}
	return nil
}

func validateInteraction(kind string, r *InteractionRequest) error {
	if r == nil {
		return problem("invalid_request", "缺少互动参数", 400)
	}
	if r.Confirm {
		_, err := hex.DecodeString(r.ActionID)
		if len(r.ActionID) != 32 || err != nil || r.Post != "" || r.CommentRef != "" || r.Text != "" || r.ImagePath != "" || r.Liked != nil || r.Favorited != nil || r.Disliked != nil || len(r.Mentions) > 0 || len(r.Emojis) > 0 {
			return problem("invalid_confirmation", "确认时只能传 action_id 和 confirm:true", 400)
		}
		return nil
	}
	if r.ActionID != "" {
		return problem("invalid_request", "action_id 仅用于确认", 400)
	}
	if _, _, err := normalizePost(r.Post); err != nil {
		return err
	}
	if kind == "like" || kind == "favorite" || kind == "comment_like" || kind == "comment_dislike" {
		fields := 0
		for _, p := range []*bool{r.Liked, r.Favorited, r.Disliked} {
			if p != nil {
				fields++
			}
		}
		correct := ((kind == "like" || kind == "comment_like") && r.Liked != nil) || (kind == "favorite" && r.Favorited != nil) || (kind == "comment_dislike" && r.Disliked != nil)
		commentKind := kind == "comment_like" || kind == "comment_dislike"
		if fields != 1 || !correct || r.Text != "" || r.ImagePath != "" || len(r.Mentions) > 0 || len(r.Emojis) > 0 || (!commentKind && r.CommentRef != "") {
			return problem("invalid_reaction", "互动类型与状态字段不匹配，或包含不相关的参数", 400)
		}
		if commentKind {
			if _, err := hex.DecodeString(r.CommentRef); len(r.CommentRef) != 32 || err != nil {
				return problem("invalid_comment_ref", "评论互动需提供有效的 comment_ref", 400)
			}
		}
		return nil
	}
	if kind != "comment" && kind != "reply" {
		return problem("invalid_action", "未知互动类型", 400)
	}
	if r.Liked != nil || r.Favorited != nil || r.Disliked != nil || utf8.RuneCountInString(r.Text) > 500 || strings.ContainsRune(r.Text, 0) || (strings.TrimSpace(r.Text) == "" && r.ImagePath == "" && len(r.Mentions) == 0 && len(r.Emojis) == 0) || len(r.Mentions) > 5 || len(r.Emojis) > 10 {
		return problem("invalid_comment", "评论需包含正文、图片、提及或表情；正文最多 500 字，最多 5 个提及、10 个表情", 400)
	}
	for _, ref := range r.Mentions {
		if _, err := hex.DecodeString(ref); len(ref) != 32 || err != nil {
			return problem("invalid_mention", "提及必须使用 get_mention_candidates 返回的 mention_ref", 400)
		}
	}
	for _, ref := range r.Emojis {
		if !validWebRef(ref) {
			return problem("invalid_emoji", "请使用 get_emoji_options 返回的 emoji_ref", 400)
		}
	}
	if kind == "reply" {
		if len(r.CommentRef) != 32 {
			return problem("invalid_comment_ref", "回复必须使用 get_comments 返回的 comment_ref", 400)
		}
		if _, err := hex.DecodeString(r.CommentRef); err != nil {
			return problem("invalid_comment_ref", "comment_ref 无效", 400)
		}
	} else if r.CommentRef != "" {
		return problem("invalid_comment", "顶层评论不能带 comment_ref，请使用 reply_comment", 400)
	}
	return nil
}
