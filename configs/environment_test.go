package configs

import (
	"path/filepath"
	"testing"
)

func TestConfigDefaultsAndPrecedence(t *testing.T) {
	empty := func(string) string { return "" }
	c, err := Parse(nil, empty)
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "127.0.0.1" || c.Port != 18070 || c.Headless || c.Stealth || c.NoSandbox {
		t.Fatalf("unsafe defaults: %+v", c)
	}
	env := func(key string) string {
		switch key {
		case "DY_PORT":
			return "18888"
		case "DY_HEADLESS":
			return "true"
		}
		return ""
	}
	c, err = Parse([]string{"--port", "18070", "--headless=false"}, env)
	if err != nil || c.Port != 18070 || c.Headless {
		t.Fatalf("precedence: %+v %v", c, err)
	}
}

func TestConfigRejectsMalformedAndOverlappingRoots(t *testing.T) {
	for _, port := range []string{"abc", "-1", "0", "65536", "18x070"} {
		if _, err := Parse(nil, func(key string) string {
			if key == "DY_PORT" {
				return port
			}
			return ""
		}); err == nil {
			t.Fatalf("accepted port %s", port)
		}
	}
	root := t.TempDir()
	for _, dirs := range [][2]string{{root, root}, {root, filepath.Join(root, "media")}, {filepath.Join(root, "data"), root}} {
		if _, err := Parse([]string{"--data-dir", dirs[0], "--media-root", dirs[1]}, func(string) string { return "" }); err == nil {
			t.Fatalf("accepted overlapping roots %v", dirs)
		}
	}
}
