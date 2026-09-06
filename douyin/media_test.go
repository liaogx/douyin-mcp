package douyin

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func samplePNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRequestValidation(t *testing.T) {
	for _, req := range []*PublishRequest{nil, {}, {Title: strings.Repeat("字", 31), VideoPath: "ok.mp4"}, {Title: "title", VideoPath: "x.mp4", ImagePaths: []string{"x.png"}}, {Confirm: true}, {Confirm: true, DraftID: strings.Repeat("a", 32), Title: "changed"}} {
		if err := validateRequest("video", req); err == nil {
			t.Fatalf("accepted invalid request: %+v", req)
		}
	}
	if err := validateRequest("video", &PublishRequest{Title: "标题", VideoPath: "a.mp4"}); err != nil {
		t.Fatal(err)
	}
	if err := validateRequest("image", &PublishRequest{Title: "标题", ImagePaths: []string{"a.png"}}); err != nil {
		t.Fatal(err)
	}
	if err := validateRequest("video", &PublishRequest{Confirm: true, DraftID: strings.Repeat("a", 32)}); err != nil {
		t.Fatal(err)
	}
}

func TestMediaBoundaryAndImmutableSnapshot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "media")
	data := filepath.Join(base, "private")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	pngData := samplePNG(t)
	original := filepath.Join(root, "photo.png")
	if err := os.WriteFile(original, pngData, 0600); err != nil {
		t.Fatal(err)
	}
	req := &PublishRequest{Title: "标题", ImagePaths: []string{"photo.png"}}
	staged, err := stageMedia(context.Background(), root, data, "image", req)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(staged.Dir)
	if err := os.WriteFile(original, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(staged.Paths[0])
	if !bytes.Equal(b, pngData) {
		t.Fatal("staged content changed with source")
	}
	if len(staged.Hashes[0]) != 64 {
		t.Fatal("missing media digest")
	}
	out := filepath.Join(base, "outside.png")
	if err := os.WriteFile(out, pngData, 0600); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink(out, filepath.Join(root, "escape.png"))
	for _, path := range []string{"../outside.png", out, "escape.png", "https://example.com/x.png", "photo.png"} {
		req.ImagePaths = []string{path}
		if _, err := stageMedia(context.Background(), root, data, "image", req); err == nil {
			t.Fatalf("accepted disallowed media %s", path)
		}
	}
}

func TestMediaCancellation(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "p.png"), samplePNG(t), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := stageMedia(ctx, root, t.TempDir(), "image", &PublishRequest{ImagePaths: []string{"p.png"}}); err == nil {
		t.Fatal("cancelled upload preparation succeeded")
	}
}

func TestLoginEvidenceIsConservative(t *testing.T) {
	positive := pageState{URL: CreatorHome, Text: "作品管理", HasAvatar: true}
	if !authenticated(positive, "session-hash") {
		t.Fatal("valid evidence rejected")
	}
	for _, s := range []pageState{{URL: CreatorHome, Text: "扫码登录", HasLogin: true, HasAvatar: true}, {URL: "https://creator.douyin.com.evil.test/creator-micro/home", HasAvatar: true}, {URL: "https://www.douyin.com/", HasAvatar: true}} {
		if authenticated(s, "session-hash") {
			t.Fatalf("false login: %+v", s)
		}
	}
	if authenticated(positive, "") {
		t.Fatal("DOM alone cannot establish login")
	}
}
