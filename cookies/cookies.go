package cookies

import (
	"encoding/json"
	"fmt"
	"github.com/go-rod/rod/lib/proto"
	"github.com/liaogx/douyin-mcp/internal/securefile"
	"strings"
	"time"
)

type Cookier interface {
	Load() ([]byte, error)
	Save([]byte) error
	Delete() error
}
type FileCookies struct{ path string }

func NewFileCookiesWithPath(path string) *FileCookies { return &FileCookies{path: path} }
func (f *FileCookies) Load() ([]byte, error)          { return securefile.Read(f.path, 4<<20) }
func (f *FileCookies) Save(data []byte) error         { return securefile.Write(f.path, data) }
func (f *FileCookies) Delete() error                  { return securefile.Delete(f.path) }

func AllowedDomain(domain string) bool {
	d := strings.TrimPrefix(strings.ToLower(domain), ".")
	return d == "douyin.com" || strings.HasSuffix(d, ".douyin.com")
}

func Encode(items []*proto.NetworkCookie) ([]byte, error) {
	params := make([]*proto.NetworkCookieParam, 0, len(items))
	for _, c := range items {
		if c == nil || !AllowedDomain(c.Domain) {
			continue
		}
		p := &proto.NetworkCookieParam{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Secure: c.Secure, HTTPOnly: c.HTTPOnly, SameSite: c.SameSite}
		if !c.Session && c.Expires > 0 {
			p.Expires = proto.TimeSinceEpoch(c.Expires)
		}
		params = append(params, p)
	}
	return json.Marshal(params)
}

func Decode(data []byte, now time.Time) ([]*proto.NetworkCookieParam, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var items []*proto.NetworkCookieParam
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("登录凭证格式损坏，请退出并重新扫码: %w", err)
	}
	result := make([]*proto.NetworkCookieParam, 0, len(items))
	for _, c := range items {
		if c == nil || !AllowedDomain(c.Domain) || c.Name == "" {
			return nil, fmt.Errorf("登录凭证包含不允许的域名或空名称")
		}
		if c.Expires > 0 && float64(c.Expires) <= float64(now.Unix()) {
			continue
		}
		c.URL = ""
		if c.Path == "" {
			c.Path = "/"
		}
		result = append(result, c)
	}
	return result, nil
}
