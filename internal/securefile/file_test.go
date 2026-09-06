package securefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateAtomicStorage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	path := filepath.Join(dir, "secret")
	if err := Write(path, []byte("synthetic-secret")); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]os.FileMode{dir: 0700, path: 0600} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Fatalf("%s mode %o", p, info.Mode().Perm())
		}
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	b, err := Read(path, 100)
	if err != nil || string(b) != "synthetic-secret" {
		t.Fatalf("read %q %v", b, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal("legacy mode was not tightened")
	}
	if err := Write(path, []byte("replacement")); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path, 2); err == nil {
		t.Fatal("oversize read accepted")
	}
	if err := Delete(path); err != nil {
		t.Fatal(err)
	}
	if err := Delete(path); err != nil {
		t.Fatal(err)
	}
}

func TestRejectSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "original")
	link := filepath.Join(dir, "link")
	if err := os.WriteFile(target, []byte("do not touch"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skip(err)
	}
	if _, err := Read(link, 100); err == nil {
		t.Fatal("read symlink accepted")
	}
	if err := Write(link, []byte("bad")); err == nil {
		t.Fatal("write symlink accepted")
	}
	if err := Delete(link); err == nil {
		t.Fatal("delete symlink accepted")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "do not touch" {
		t.Fatal("target modified")
	}
}
