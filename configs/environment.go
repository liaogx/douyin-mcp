package configs

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is immutable after startup. Secrets are never written to logs.
type Config struct {
	Host, BinPath, Proxy, UserAgent, DataDir, MediaRoot, AuthToken string
	Port                                                           int
	Headless, Stealth, NoSandbox                                   bool
	OperationTimeout                                               time.Duration
}

func Parse(args []string, getenv func(string) string) (Config, error) {
	c := Config{Host: "127.0.0.1", Port: 18070, DataDir: "./data", MediaRoot: "./media", OperationTimeout: 10 * time.Minute}
	fs := flag.NewFlagSet("douyin-mcp", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	envString := func(name, fallback string) string {
		if v := getenv(name); v != "" {
			return v
		}
		return fallback
	}
	var err error
	envBool := func(name string, fallback bool) bool {
		if v := getenv(name); v != "" {
			b, e := strconv.ParseBool(v)
			if e != nil {
				err = fmt.Errorf("%s 必须是 true/false", name)
			}
			return b
		}
		return fallback
	}
	port := c.Port
	if v := getenv("DY_PORT"); v != "" {
		var e error
		port, e = strconv.Atoi(v)
		if e != nil {
			return c, errors.New("DY_PORT 必须是整数")
		}
	}
	fs.StringVar(&c.Host, "host", envString("DY_HOST", c.Host), "HTTP bind address")
	fs.IntVar(&c.Port, "port", port, "HTTP port")
	fs.StringVar(&c.BinPath, "bin", getenv("ROD_BROWSER_BIN"), "Chrome/Chromium executable")
	fs.StringVar(&c.Proxy, "proxy", getenv("DY_PROXY"), "Optional proxy URL without credentials")
	fs.StringVar(&c.UserAgent, "user-agent", getenv("DY_USER_AGENT"), "Empty keeps the real browser default")
	fs.StringVar(&c.DataDir, "data-dir", envString("DY_DATA_DIR", c.DataDir), "Dedicated private state directory")
	fs.StringVar(&c.MediaRoot, "media-root", envString("DY_MEDIA_ROOT", c.MediaRoot), "Only files under this directory may be uploaded")
	fs.BoolVar(&c.Headless, "headless", envBool("DY_HEADLESS", false), "Hide the dedicated browser window")
	fs.BoolVar(&c.Stealth, "stealth", envBool("DY_STEALTH", false), "Optional static stealth script; not a captcha bypass")
	fs.BoolVar(&c.NoSandbox, "no-sandbox", envBool("DY_NO_SANDBOX", false), "Insecure fallback for incompatible container hosts")
	fs.DurationVar(&c.OperationTimeout, "timeout", c.OperationTimeout, "Maximum operation duration")
	if err != nil {
		return c, err
	}
	if err = fs.Parse(args); err != nil {
		return c, err
	}
	if len(fs.Args()) != 0 {
		return c, errors.New("不支持位置参数")
	}
	c.AuthToken = strings.TrimSpace(getenv("DY_AUTH_TOKEN"))
	if c.AuthToken != "" && len(c.AuthToken) < 32 {
		return c, errors.New("DY_AUTH_TOKEN 至少需要 32 个字符")
	}
	if c.Port < 1 || c.Port > 65535 {
		return c, errors.New("端口必须在 1–65535 之间")
	}
	if c.Host != "localhost" && net.ParseIP(c.Host) == nil {
		return c, errors.New("host 必须是 IP 地址或 localhost")
	}
	if c.OperationTimeout < 30*time.Second || c.OperationTimeout > 30*time.Minute {
		return c, errors.New("timeout 必须在 30s–30m 之间")
	}
	c.DataDir, err = filepath.Abs(c.DataDir)
	if err != nil {
		return c, err
	}
	c.MediaRoot, err = filepath.Abs(c.MediaRoot)
	if err != nil {
		return c, err
	}
	if within(c.MediaRoot, c.DataDir) || within(c.DataDir, c.MediaRoot) {
		return c, errors.New("素材目录与私有数据目录不得重叠")
	}
	wd, _ := os.Getwd()
	homeDir, _ := os.UserHomeDir()
	for _, dir := range []string{c.DataDir, c.MediaRoot} {
		if dir == filepath.Dir(dir) || dir == homeDir || dir == wd {
			return c, errors.New("请使用专用子目录，不要使用主目录、项目根目录或文件系统根目录")
		}
	}
	return c, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
