package cookies

import (
	"github.com/go-rod/rod/lib/proto"
	"testing"
	"time"
)

func TestCookieRoundTripAndDomainFilter(t *testing.T) {
	items := []*proto.NetworkCookie{
		{Name: "sessionid", Value: "synthetic", Domain: ".douyin.com", Path: "/", Secure: true, HTTPOnly: true, Session: true, SameSite: proto.NetworkCookieSameSiteLax},
		{Name: "unrelated", Value: "not-saved", Domain: "example.com"},
	}
	b, err := Encode(items)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(b, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "sessionid" || !got[0].HTTPOnly || got[0].SameSite != proto.NetworkCookieSameSiteLax {
		t.Fatalf("unexpected roundtrip: %+v", got)
	}
}

func TestRejectForeignAndExpiredCookies(t *testing.T) {
	for _, domain := range []string{"evil-douyin.com", "douyin.com.evil.test", "localhost", ""} {
		if AllowedDomain(domain) {
			t.Fatalf("accepted %s", domain)
		}
	}
	if _, err := Decode([]byte(`[{"name":"x","domain":"evil.test"}]`), time.Now()); err == nil {
		t.Fatal("foreign cookie accepted")
	}
	if _, err := Decode([]byte(`not-json`), time.Now()); err == nil {
		t.Fatal("corrupt cookie accepted")
	}
	got, err := Decode([]byte(`[{"name":"x","domain":".douyin.com","expires":100}]`), time.Now())
	if err != nil || len(got) != 0 {
		t.Fatal("expired cookie restored")
	}
}
