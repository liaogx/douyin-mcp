package douyin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestInteractionReceiptsFailClosedAcrossRestarts(t *testing.T) {
	s := &WebService{dataDir: t.TempDir()}
	id, err := webNonce()
	if err != nil {
		t.Fatal(err)
	}
	r := &InteractionResult{ActionID: id, Kind: "comment", Stage: "submitting"}
	if err := s.interactionJournal(r); err != nil {
		t.Fatal(err)
	}
	fresh := &WebService{dataDir: s.dataDir}
	got, err := fresh.interactionPrior("comment", id)
	if err == nil || got == nil || got.Stage != "unknown" || got.Success {
		t.Fatalf("ambiguous receipt accepted: %+v %v", got, err)
	}
	if _, err := fresh.interactionPrior("like", id); err == nil {
		t.Fatal("wrong tool accepted another action receipt")
	}
	r.Stage = "completed"
	r.Success = true
	if err := s.interactionJournal(r); err != nil {
		t.Fatal(err)
	}
	got, err = fresh.interactionPrior("comment", id)
	if err != nil || !got.Success {
		t.Fatal("completed receipt not restored", err)
	}
	if _, err := fresh.interactionPrior("comment", "../../secrets"); err == nil {
		t.Fatal("unsafe ID accepted")
	}
}

func TestUnchangedReceiptFailureClosesAction(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	id, err := webNonce()
	if err != nil {
		t.Fatal(err)
	}
	d := &interactionDraft{id: id, kind: "like", result: &InteractionResult{ActionID: id, Kind: "like"}}
	s := &WebService{dataDir: blocked, active: d}
	if _, err := s.completeInteraction(d, "unchanged", "already off"); err == nil {
		t.Fatal("expected receipt failure")
	}
	if !d.attempted || d.result.ExpiresAt != nil {
		t.Fatal("action remained reusable")
	}
	s.dataDir = t.TempDir()
	if _, err := s.confirmInteraction(context.Background(), "like", id); err == nil {
		t.Fatal("receipt failure allowed another attempt")
	}
}
