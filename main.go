package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/liaogx/douyin-mcp/configs"
	"github.com/liaogx/douyin-mcp/internal/securefile"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func run() error {
	c, err := configs.Parse(os.Args[1:], os.Getenv)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := securefile.EnsureDir(c.DataDir); err != nil {
		return err
	}
	if err := os.MkdirAll(c.MediaRoot, 0700); err != nil {
		return err
	}
	// Resolve configured roots once so aliases cannot make the credential directory uploadable.
	dataReal, err := filepath.EvalSymlinks(c.DataDir)
	if err != nil {
		return err
	}
	mediaReal, err := filepath.EvalSymlinks(c.MediaRoot)
	if err != nil {
		return err
	}
	c.DataDir = dataReal
	c.MediaRoot = mediaReal
	if _, err := configs.Parse([]string{"--data-dir", dataReal, "--media-root", mediaReal}, func(string) string { return "" }); err != nil {
		return err
	}
	token, err := authToken(c)
	if err != nil {
		return err
	}
	service := NewDouyinService(c)
	defer service.Close()
	server := NewAppServer(service, c, token)
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	slog.Info("抖音发布服务已启动", "address", server.Addr, "version", version)
	if c.AuthToken == "" {
		slog.Info("客户端需使用本地令牌文件中的 Bearer token；请勿分享此文件", "token_file", filepath.Join(c.DataDir, "api-token"))
	}
	if c.NoSandbox {
		slog.Warn("浏览器沙箱已显式关闭，请仅在受隔离的容器中使用")
	}
	slog.Info("首次登录操作才启动专用浏览器；只允许上传配置的素材目录", "media_root", c.MediaRoot, "stealth", c.Stealth)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	stopped := make(chan error, 1)
	go func() { stopped <- server.Serve(listener) }()
	select {
	case err := <-stopped:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		service.cancel()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "启动/运行失败:", err)
		os.Exit(1)
	}
}
