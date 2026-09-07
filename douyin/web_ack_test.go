package douyin

import (
	"net/url"
	"strings"
	"testing"

	"github.com/go-rod/rod/lib/proto"
)

func syntheticAckDraft(kind string) *interactionDraft {
	yes := true
	return &interactionDraft{kind: kind, postID: "7665228646013364602", actorURL: "https://www.douyin.com/user/local-actor",
		request: InteractionRequest{Liked: &yes, Favorited: &yes}, result: &InteractionResult{Text: "合成测试评论"}}
}

const syntheticCommentAck = `{"status_code":0,"comment":{"cid":"7000000000000000001","aweme_id":"7665228646013364602","text":"合成测试评论","user":{"sec_uid":"local-actor"}}}`

func TestAckRequestMustBindAction(t *testing.T) {
	for _, kind := range []string{"comment", "like", "favorite"} {
		d := syntheticAckDraft(kind)
		body := url.Values{"aweme_id": {d.postID}, "type": {"1"}, "text": {d.result.Text}}.Encode()
		base := proto.NetworkRequest{URL: "https://www.douyin.com" + ackPath(d), Method: "POST", PostData: body}
		if !matchesAckRequest(d, &base) {
			t.Fatal("valid request not recognized", kind)
		}
		for _, mode := range []string{"host", "credentials", "method", "post", "duplicate", "intent", "oversize"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				r := base
				switch mode {
				case "host":
					r.URL = strings.Replace(r.URL, "www.douyin.com", "www.douyin.com.invalid", 1)
				case "credentials":
					r.URL = strings.Replace(r.URL, "https://", "https://fake-user@", 1)
				case "method":
					r.Method = "GET"
				case "post":
					r.PostData = strings.Replace(r.PostData, d.postID, "7000000000000000002", 1)
				case "duplicate":
					r.URL += "?aweme_id=" + d.postID
				case "intent":
					if kind == "comment" {
						r.PostData += "&reply_id=7000000000000000002"
					} else {
						r.PostData = strings.Replace(r.PostData, "type=1", "type=0", 1)
					}
				case "oversize":
					r.PostData += strings.Repeat("x", 33<<10)
				}
				if matchesAckRequest(d, &r) {
					t.Fatal("unbound request accepted")
				}
			})
		}
	}
}

func TestAckHTTP200IsNotBusinessSuccess(t *testing.T) {
	d := syntheticAckDraft("comment")
	if id, err := parseActionAck(d, 200, []byte(syntheticCommentAck)); err != nil || id != "7000000000000000001" {
		t.Fatal("valid bound comment rejected", err)
	}
	for name, body := range map[string]string{
		"html": "<html>verification</html>", "missing_status": `{}`, "null_status": `{"status_code":null}`,
		"rejected": `{"status_code":8}`, "no_comment": `{"status_code":0}`, "wrong_id": strings.Replace(syntheticCommentAck, "7000000000000000001", "", 1),
		"wrong_post":  strings.Replace(syntheticCommentAck, d.postID, "7000000000000000002", 1),
		"wrong_actor": strings.Replace(syntheticCommentAck, "local-actor", "different-actor", 1),
		"wrong_text":  strings.Replace(syntheticCommentAck, d.result.Text, "different text", 1),
		"reply":       strings.Replace(syntheticCommentAck, `"cid":`, `"reply_id":"7000000000000000002","cid":`, 1),
		"oversize":    strings.Repeat(" ", maxAckBody) + syntheticCommentAck,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseActionAck(d, 200, []byte(body)); err == nil {
				t.Fatal("HTTP 200 incorrectly accepted")
			}
		})
	}
	if _, err := parseActionAck(d, 503, []byte(syntheticCommentAck)); err == nil {
		t.Fatal("HTTP failure accepted")
	}
	if _, err := parseActionAck(syntheticAckDraft("like"), 200, []byte(`{"status_code":0}`)); err != nil {
		t.Fatal(err)
	}
}
