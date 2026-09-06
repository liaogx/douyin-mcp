package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liaogx/douyin-mcp/douyin"
)

func (f *fakeOps) CheckWebLoginStatus(c context.Context) (*douyin.LoginResult, error) {
	return f.CheckLoginStatus(c)
}
func (f *fakeOps) GetWebLoginQRCode(c context.Context) (*douyin.LoginResult, error) {
	return f.GetLoginQRCode(c)
}
func (f *fakeOps) SearchPosts(context.Context, *douyin.SearchRequest) (*douyin.SearchResult, error) {
	f.calls++
	return &douyin.SearchResult{}, f.fail
}
func (f *fakeOps) GetSearchFilters(context.Context, *douyin.SearchRequest) (*douyin.SearchFiltersResult, error) {
	f.calls++
	return &douyin.SearchFiltersResult{}, f.fail
}
func (f *fakeOps) GetPostDetail(context.Context, *douyin.PostRequest) (*douyin.PostDetail, error) {
	f.calls++
	return &douyin.PostDetail{}, f.fail
}
func (f *fakeOps) GetComments(context.Context, *douyin.CommentsRequest) (*douyin.CommentsResult, error) {
	f.calls++
	return &douyin.CommentsResult{}, f.fail
}
func (f *fakeOps) Interact(_ context.Context, kind string, _ *douyin.InteractionRequest) (*douyin.InteractionResult, error) {
	f.calls++
	return &douyin.InteractionResult{Kind: kind, Stage: "ready"}, f.fail
}
func (f *fakeOps) GetMentionCandidates(context.Context, *douyin.MentionRequest) (*douyin.MentionResult, error) {
	f.calls++
	return &douyin.MentionResult{}, f.fail
}
func (f *fakeOps) GetEmojiOptions(context.Context, *douyin.PostRequest) (*douyin.EmojiResult, error) {
	f.calls++
	return &douyin.EmojiResult{}, f.fail
}

func TestWebRoutesUseSameSecurityAndStrictJSON(t *testing.T) {
	paths := []string{"search", "search/filters", "post", "comments", "mentions", "emojis"}
	for name := range interactions {
		paths = append(paths, name)
	}
	f := &fakeOps{}
	server := testServer(f)
	for _, path := range paths {
		for _, authorized := range []bool{false, true} {
			r := httptest.NewRequest("POST", "http://127.0.0.1:18070/api/v1/web/"+path, strings.NewReader(`{"unexpected_extra":"not allowed"}`))
			r.Header.Set("Content-Type", "application/json")
			want := 401
			if authorized {
				r.Header.Set("Authorization", "Bearer "+testToken)
				want = 400
			}
			w := httptest.NewRecorder()
			server.Handler.ServeHTTP(w, r)
			if w.Code != want {
				t.Fatalf("%s authorized=%v: got %d want %d", path, authorized, w.Code, want)
			}
		}
	}
	if f.calls != 0 {
		t.Fatal("rejected requests reached web operations")
	}
}

func TestWebToolRegistrationsAndLoginSurface(t *testing.T) {
	f := &fakeOps{}
	s := testServer(f)
	listed := rpc(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	data, _ := json.Marshal(listed)
	for _, name := range []string{"search_posts", "get_search_filters", "get_post_detail", "get_comments", "get_mention_candidates", "get_emoji_options", "set_post_like", "set_post_favorite", "comment_post", "reply_comment", "set_comment_like", "set_comment_dislike"} {
		if !strings.Contains(string(data), `"name":"`+name+`"`) {
			t.Fatalf("missing %s", name)
		}
	}
	result := rpc(t, s, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"check_login_status","arguments":{"surface":"web"}}}`)
	data, _ = json.Marshal(result)
	if !strings.Contains(string(data), "logged_in") || f.calls != 1 {
		t.Fatalf("web login not dispatched: %s", data)
	}
	result = rpc(t, s, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"check_login_status","arguments":{"surface":"sms"}}}`)
	data, _ = json.Marshal(result)
	if !strings.Contains(string(data), `"isError":true`) || f.calls != 1 {
		t.Fatal("unknown login surface accepted")
	}
}
