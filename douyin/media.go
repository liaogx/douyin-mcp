package douyin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/liaogx/douyin-mcp/internal/securefile"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type stagedMedia struct {
	Dir                  string
	Paths, Names, Hashes []string
}

func validateRequest(kind string, req *PublishRequest) error {
	if req == nil {
		return problem("invalid_request", "缺少发布参数", 400)
	}
	if req.Confirm {
		if len(req.DraftID) != 32 || req.Title != "" || req.Description != "" || req.VideoPath != "" || len(req.ImagePaths) != 0 {
			return problem("invalid_confirmation", "确认发布时只能提供 draft_id 和 confirm:true，不能修改素材或文案", 400)
		}
		if _, err := hex.DecodeString(req.DraftID); err != nil {
			return problem("invalid_draft", "draft_id 无效", 400)
		}
		return nil
	}
	if req.DraftID != "" {
		return problem("invalid_request", "draft_id 仅用于确认发布", 400)
	}
	if strings.TrimSpace(req.Title) == "" || utf8.RuneCountInString(req.Title) > 30 || strings.ContainsAny(req.Title, "\r\n\x00") {
		return problem("invalid_title", "标题需为 1–30 字，且不能包含换行", 400)
	}
	if utf8.RuneCountInString(req.Description) > 1000 || strings.ContainsRune(req.Description, 0) {
		return problem("invalid_description", "正文不能超过 1000 字或包含空字符", 400)
	}
	if kind == "video" && (req.VideoPath == "" || len(req.ImagePaths) != 0) {
		return problem("invalid_media", "视频发布仅接受一个 video_path", 400)
	}
	if kind == "image" && (req.VideoPath != "" || len(req.ImagePaths) == 0 || len(req.ImagePaths) > 35) {
		return problem("invalid_media", "图文发布需要 1–35 个 image_paths，不能同时提供视频", 400)
	}
	return nil
}

func stageMedia(ctx context.Context, rootDir, dataDir, kind string, req *PublishRequest) (*stagedMedia, error) {
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, problem("media_root", "素材目录不存在或无法打开", 400)
	}
	defer root.Close()
	stageRoot := filepath.Join(dataDir, "staging")
	if err := securefile.EnsureDir(stageRoot); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(stageRoot, "upload-*")
	if err != nil {
		return nil, err
	}
	result := &stagedMedia{Dir: dir}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(dir)
		}
	}()
	paths := req.ImagePaths
	if kind == "video" {
		paths = []string{req.VideoPath}
	}
	for i, path := range paths {
		if strings.Contains(path, "://") {
			return nil, problem("invalid_path", "仅接受素材目录中的本地文件，不接受网址", 400)
		}
		rel := filepath.Clean(path)
		if filepath.IsAbs(path) {
			rel, err = filepath.Rel(rootDir, path)
			if err != nil {
				return nil, err
			}
		}
		if !filepath.IsLocal(rel) {
			return nil, problem("outside_media_root", "文件不在授权的素材目录中", 400)
		}
		// OpenRoot also prevents symlinks and racing path components escaping root.
		src, err := root.Open(rel)
		if err != nil {
			return nil, problem("invalid_path", "无法读取素材，或符号链接越过了素材目录边界", 400)
		}
		name := filepath.Base(rel)
		ext := strings.ToLower(filepath.Ext(name))
		dest := filepath.Join(dir, fmt.Sprintf("%02d%s", i, ext))
		hash, err := copyMedia(ctx, src, dest, kind, ext)
		src.Close()
		if err != nil {
			return nil, err
		}
		result.Paths = append(result.Paths, dest)
		result.Names = append(result.Names, name)
		result.Hashes = append(result.Hashes, hash)
	}
	ok = true
	return result, nil
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func copyMedia(ctx context.Context, src *os.File, dest, kind, ext string) (string, error) {
	i, err := src.Stat()
	if err != nil {
		return "", err
	}
	limit := int64(20 << 20)
	if kind == "video" {
		limit = 2 << 30
	}
	if !i.Mode().IsRegular() || i.Size() == 0 || i.Size() > limit {
		return "", problem("invalid_media", "素材必须是非空普通文件；图片最大 20MB，视频最大 2GB", 400)
	}
	head := make([]byte, 512)
	n, err := src.Read(head)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	head = head[:n]
	if kind == "video" {
		if (ext != ".mp4" && ext != ".mov") || len(head) < 12 || string(head[4:8]) != "ftyp" {
			return "", problem("invalid_media", "视频仅支持真实的 MP4/MOV 文件，请先转为 MP4", 400)
		}
	} else {
		mime := http.DetectContentType(head)
		valid := ((ext == ".jpg" || ext == ".jpeg") && mime == "image/jpeg") || (ext == ".png" && mime == "image/png") || (ext == ".webp" && mime == "image/webp")
		if !valid {
			return "", problem("invalid_media", "图片仅支持与扩展名一致的 JPEG/PNG/WebP 文件", 400)
		}
	}
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	defer out.Close()
	hash := sha256.New()
	nbytes, err := io.Copy(io.MultiWriter(out, hash), io.LimitReader(contextReader{ctx, src}, limit+1))
	if err != nil {
		return "", err
	}
	after, err := src.Stat()
	if err != nil {
		return "", err
	}
	if nbytes != i.Size() || after.Size() != i.Size() || !after.ModTime().Equal(i.ModTime()) {
		return "", problem("media_changed", "复制过程中素材发生变化，请重试准备预览", 409)
	}
	if err := out.Sync(); err != nil {
		return "", err
	}
	if err := out.Chmod(0400); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
