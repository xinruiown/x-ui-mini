package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xinruiown/x-ui-mini/internal/store"
	"github.com/xinruiown/x-ui-mini/internal/xraycfg"
)

func TestOpenAndHash(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	s, err := store.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg := s.Get()
	if cfg.Panel.Listen != "127.0.0.1" {
		t.Fatalf("listen %s", cfg.Panel.Listen)
	}
	if store.HashPassword("a", "b") == store.HashPassword("a", "c") {
		t.Fatal("hash collision")
	}
}

func TestXrayBuild(t *testing.T) {
	cfg := store.Config{
		Nodes: []store.Node{{
			ID: "n1", Protocol: "vless-reality", Port: 443, UUID: "u", Enabled: true,
			Reality: store.Reality{PrivateKey: "p", PublicKey: "P", ShortID: "abcd"},
		}},
		SOCKS: &store.SOCKS{Listen: "127.0.0.1", Port: 1080, Enabled: true},
	}
	b, err := xraycfg.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 20 {
		t.Fatal("empty")
	}
	if xraycfg.Share(cfg.Nodes[0], "1.2.3.4") == "" {
		t.Fatal("share")
	}
}

func TestConfigPersist(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	s, err := store.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(c *store.Config) error {
		c.Panel.PublicHost = "example.com"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(p)
	if err != nil || len(raw) == 0 {
		t.Fatal("empty config")
	}
}
