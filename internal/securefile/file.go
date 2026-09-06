// Package securefile protects files from other OS users, not same-user malware.
package securefile

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func EnsureDir(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(dir, 0700)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("私有数据目录必须是实际目录，不能是符号链接")
	}
	return os.Chmod(dir, 0700)
}

func check(path string) (os.FileInfo, error) {
	i, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !i.Mode().IsRegular() {
		return nil, errors.New("私有文件必须是普通文件，不能是符号链接")
	}
	return i, nil
}

func Read(path string, limit int64) ([]byte, error) {
	before, err := check(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	f, err := r.Open(filepath.Base(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, after) {
		return nil, errors.New("私有文件在读取时发生变化")
	}
	if err := f.Chmod(0600); err != nil {
		return nil, err
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, errors.New("私有文件过大")
	}
	return b, err
}

func Write(path string, data []byte) error {
	if _, err := check(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dir := filepath.Dir(path)
	if err := EnsureDir(dir); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	defer f.Close()
	if err := f.Chmod(0600); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("同步私有目录失败: %w", err)
	}
	return nil
}

func Delete(path string) error {
	if _, err := check(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return os.Remove(path)
}
