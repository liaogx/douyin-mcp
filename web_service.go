package main

import (
	"context"
	"github.com/liaogx/douyin-mcp/douyin"
)

type WebOperations interface {
	CheckWebLoginStatus(context.Context) (*douyin.LoginResult, error)
	GetWebLoginQRCode(context.Context) (*douyin.LoginResult, error)
	SearchPosts(context.Context, *douyin.SearchRequest) (*douyin.SearchResult, error)
	GetSearchFilters(context.Context, *douyin.SearchRequest) (*douyin.SearchFiltersResult, error)
	GetPostDetail(context.Context, *douyin.PostRequest) (*douyin.PostDetail, error)
	GetComments(context.Context, *douyin.CommentsRequest) (*douyin.CommentsResult, error)
	Interact(context.Context, string, *douyin.InteractionRequest) (*douyin.InteractionResult, error)
	GetMentionCandidates(context.Context, *douyin.MentionRequest) (*douyin.MentionResult, error)
	GetEmojiOptions(context.Context, *douyin.PostRequest) (*douyin.EmojiResult, error)
}

func webCall[T any](ctx context.Context, s *DouyinService, fn func(context.Context, *douyin.WebService) (*T, error)) (*T, error) {
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer done()
	if err := s.ensure(ctx); err != nil {
		return nil, err
	}
	return fn(ctx, s.web)
}
func (s *DouyinService) CheckWebLoginStatus(ctx context.Context) (*douyin.LoginResult, error) {
	return webCall(ctx, s, func(ctx context.Context, w *douyin.WebService) (*douyin.LoginResult, error) {
		return w.CheckLoginStatus(ctx)
	})
}
func (s *DouyinService) GetWebLoginQRCode(ctx context.Context) (*douyin.LoginResult, error) {
	return webCall(ctx, s, func(ctx context.Context, w *douyin.WebService) (*douyin.LoginResult, error) {
		return w.GetLoginQRCode(ctx)
	})
}
func (s *DouyinService) SearchPosts(ctx context.Context, r *douyin.SearchRequest) (*douyin.SearchResult, error) {
	return webCall(ctx, s, func(ctx context.Context, w *douyin.WebService) (*douyin.SearchResult, error) {
		return w.SearchPosts(ctx, r)
	})
}
func (s *DouyinService) GetSearchFilters(ctx context.Context, r *douyin.SearchRequest) (*douyin.SearchFiltersResult, error) {
	return webCall(ctx, s, func(ctx context.Context, w *douyin.WebService) (*douyin.SearchFiltersResult, error) {
		return w.GetSearchFilters(ctx, r)
	})
}
func (s *DouyinService) GetPostDetail(ctx context.Context, r *douyin.PostRequest) (*douyin.PostDetail, error) {
	return webCall(ctx, s, func(ctx context.Context, w *douyin.WebService) (*douyin.PostDetail, error) {
		return w.GetPostDetail(ctx, r)
	})
}
func (s *DouyinService) GetComments(ctx context.Context, r *douyin.CommentsRequest) (*douyin.CommentsResult, error) {
	return webCall(ctx, s, func(ctx context.Context, w *douyin.WebService) (*douyin.CommentsResult, error) {
		return w.GetComments(ctx, r)
	})
}
func (s *DouyinService) Interact(ctx context.Context, kind string, r *douyin.InteractionRequest) (*douyin.InteractionResult, error) {
	return webCall(ctx, s, func(ctx context.Context, w *douyin.WebService) (*douyin.InteractionResult, error) {
		return w.Interact(ctx, kind, r)
	})
}
func (s *DouyinService) GetMentionCandidates(ctx context.Context, r *douyin.MentionRequest) (*douyin.MentionResult, error) {
	return webCall(ctx, s, func(ctx context.Context, w *douyin.WebService) (*douyin.MentionResult, error) {
		return w.GetMentionCandidates(ctx, r)
	})
}
func (s *DouyinService) GetEmojiOptions(ctx context.Context, r *douyin.PostRequest) (*douyin.EmojiResult, error) {
	return webCall(ctx, s, func(ctx context.Context, w *douyin.WebService) (*douyin.EmojiResult, error) {
		return w.GetEmojiOptions(ctx, r)
	})
}

func loginForSurface(ctx context.Context, s Operations, r douyin.LoginRequest, qr bool) (*douyin.LoginResult, error) {
	if r.Surface == "" || r.Surface == "creator" {
		if qr {
			return s.GetLoginQRCode(ctx)
		}
		return s.CheckLoginStatus(ctx)
	}
	if r.Surface != "web" {
		return nil, &douyin.Error{Code: "invalid_surface", Message: "surface 仅支持 creator 或 web", Status: 400}
	}
	w, ok := s.(WebOperations)
	if !ok {
		return nil, &douyin.Error{Code: "unavailable", Message: "网页版服务不可用", Status: 503}
	}
	if qr {
		return w.GetWebLoginQRCode(ctx)
	}
	return w.CheckWebLoginStatus(ctx)
}
