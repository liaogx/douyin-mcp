package douyin

import "testing"

func TestWebPostURLBoundary(t *testing.T) {
	for _, raw := range []string{"7648320240313828858", "https://www.douyin.com/note/7648320240313828858", "https://www.douyin.com/video/7648320240313828858?share=1", "https://www.douyin.com/jingxuan/search/深圳找对象?modal_id=7648320240313828858&type=general&aid=tracking"} {
		id, target, err := normalizePost(raw)
		if err != nil || id != "7648320240313828858" || target == "" {
			t.Fatalf("%s => %s %s %v", raw, id, target, err)
		}
	}
	for _, raw := range []string{"file:///etc/passwd", "http://www.douyin.com/video/7648320240313828858", "https://douyin.com.evil.test/video/7648320240313828858", "https://www.douyin.com:443/video/7648320240313828858", "https://a@www.douyin.com/video/7648320240313828858", "https://www.douyin.com/..%2fvideo/7648320240313828858", "https://www.douyin.com/video/not-an-id", "https://v.douyin.com/abc", "https://www.douyin.com/user/abc?modal_id=7648320240313828858", "https://127.0.0.1/video/7648320240313828858"} {
		if _, _, err := normalizePost(raw); err == nil {
			t.Fatalf("unsafe URL accepted: %s", raw)
		}
	}
}

func TestWebInteractionValidation(t *testing.T) {
	like := true
	for _, tc := range []struct {
		kind string
		r    InteractionRequest
	}{
		{"like", InteractionRequest{Post: "7648320240313828858", Liked: &like}},
		{"comment", InteractionRequest{Post: "7648320240313828858", Text: "不错"}},
		{"comment", InteractionRequest{Post: "7648320240313828858", ImagePath: "hello.png"}},
		{"reply", InteractionRequest{Post: "7648320240313828858", CommentRef: "0123456789abcdef0123456789abcdef", Text: "不错"}},
		{"comment", InteractionRequest{ActionID: "0123456789abcdef0123456789abcdef", Confirm: true}},
	} {
		if err := validateInteraction(tc.kind, &tc.r); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		kind string
		r    InteractionRequest
	}{
		{"like", InteractionRequest{Post: "7648320240313828858"}},
		{"comment", InteractionRequest{Post: "7648320240313828858"}},
		{"reply", InteractionRequest{Post: "7648320240313828858", Text: "不错"}},
		{"comment", InteractionRequest{ActionID: "0123456789abcdef0123456789abcdef", Confirm: true, Text: "changed"}},
		{"like", InteractionRequest{ActionID: "0123456789abcdef0123456789abcdef", Confirm: true, Liked: &like}},
		{"comment", InteractionRequest{ActionID: "../../../../../../../../../../..", Confirm: true}},
	} {
		if err := validateInteraction(tc.kind, &tc.r); err == nil {
			t.Fatalf("accepted invalid %+v", tc)
		}
	}
}

func TestWebSearchLimits(t *testing.T) {
	if err := validateSearch(&SearchRequest{Query: "猫咪", Filters: []FilterChoice{{Group: "排序依据", Option: "最新发布"}}}); err != nil {
		t.Fatal(err)
	}
	for _, r := range []SearchRequest{{Query: ""}, {Query: "a", Tab: "user"}, {Query: "a", Limit: 51}, {Query: "a", MaxScrolls: 6}, {Query: "a", Filters: []FilterChoice{{Group: "排序依据", Option: "综合排序"}, {Group: "排序依据", Option: "最新发布"}}}} {
		if err := validateSearch(&r); err == nil {
			t.Fatalf("invalid search accepted: %+v", r)
		}
	}
}
