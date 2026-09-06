package douyin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/liaogx/douyin-mcp/internal/securefile"
)

func webNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func webDigest(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (s *WebService) interactionJournal(r *InteractionResult) error {
	if r == nil || !validWebRef(r.ActionID) {
		return errors.New("无效的互动回执标识")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return securefile.Write(filepath.Join(s.dataDir, "interaction-receipts", r.ActionID+".json"), b)
}

func validWebRef(ref string) bool {
	b, err := hex.DecodeString(ref)
	return err == nil && len(b) == 16 && len(ref) == 32
}

// A durable 'submitting' record means the click may have reached the server.
// Neither a retry nor a process restart is permission to send it again.
func (s *WebService) interactionPrior(kind, id string) (*InteractionResult, error) {
	if !validWebRef(id) {
		return nil, problem("invalid_confirmation", "action_id 无效", 400)
	}
	b, err := securefile.Read(filepath.Join(s.dataDir, "interaction-receipts", id+".json"), 1<<20)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	var r InteractionResult
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if r.ActionID != id || r.Kind != kind {
		return nil, problem("receipt_mismatch", "互动回执标识或类型不一致，未再次操作", 409)
	}
	if (r.Stage == "completed" || r.Stage == "unchanged") && r.Success {
		return &r, nil
	}
	r.Stage = "unknown"
	r.Success = false
	r.Message = "此操作已有一次执行记录，但结果未确认；请到抖音核对，不会再次点击或发送"
	return &r, problem("interaction_unknown", r.Message, 409)
}

func (s *WebService) uncertainInteraction(d *interactionDraft, cause error) (*InteractionResult, error) {
	d.result.Stage = "unknown"
	d.result.Success = false
	d.result.ExpiresAt = nil
	d.result.Message = "已经尝试一次操作，但无法确认抖音结果。请人工核对；同一 action_id 不会重试，勿另建操作盲目重发"
	_ = s.interactionJournal(d.result)
	return d.result, problem("interaction_unknown", d.result.Message+": "+cause.Error(), 409)
}
