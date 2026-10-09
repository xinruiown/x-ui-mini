package xraycfg_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/xinruiown/x-ui-mini/internal/store"
	"github.com/xinruiown/x-ui-mini/internal/xraycfg"
)

func TestShareVLESSUsesHostAndKeys(t *testing.T) {
	n := store.Node{
		Name:     "n1",
		Protocol: "vless-reality",
		Port:     443,
		UUID:     "11111111-1111-4111-8111-111111111111",
		Reality: store.Reality{
			PublicKey:   "AbC",
			ShortID:     "abcd1234",
			ServerNames: []string{"www.cloudflare.com"},
			Fingerprint: "chrome",
		},
	}
	s := xraycfg.Share(n, "206.237.120.223")
	if strings.Contains(s, "YOUR_IP") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "206.237.120.223") || !strings.Contains(s, "pbk=AbC") || !strings.Contains(s, "sid=abcd1234") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "security=reality") || !strings.Contains(s, "flow=xtls-rprx-vision") {
		t.Fatal(s)
	}
}

func TestShareEmptyHostRejected(t *testing.T) {
	n := store.Node{Protocol: "vless-reality", Port: 443, UUID: "u", Reality: store.Reality{PublicKey: "k", ShortID: "ab"}}
	s := xraycfg.Share(n, "")
	if s == "" || strings.Contains(s, "YOUR_IP") {
		t.Fatalf("share %q", s)
	}
}

func TestBuildRealitySniffingAndPrivateKey(t *testing.T) {
	cfg := store.Config{Nodes: []store.Node{{
		ID: "n1", Protocol: "vless-reality", Port: 443, UUID: "u", Enabled: true,
		Reality: store.Reality{PrivateKey: "priv", PublicKey: "pub", ShortID: "abcd", Dest: "www.cloudflare.com:443", ServerNames: []string{"www.cloudflare.com"}},
	}}}
	b, err := xraycfg.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(doc)
	s := string(raw)
	if !strings.Contains(s, `"privateKey":"priv"`) {
		t.Fatal(s)
	}
	if !strings.Contains(s, `"sniffing"`) {
		t.Fatal("missing sniffing")
	}
}

func TestBuildSOCKSAndHTTPAuth(t *testing.T) {
	cfg := store.Config{
		SOCKS: &store.SOCKS{Listen: "0.0.0.0", Port: 1080, User: "u", Pass: "p", Enabled: true, UDP: true},
		HTTP:  &store.HTTP{Listen: "127.0.0.1", Port: 8080, User: "hu", Pass: "hp", Enabled: true},
	}
	b, err := xraycfg.Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"protocol": "socks"`) || !strings.Contains(s, `"protocol": "http"`) {
		t.Fatal(s)
	}
	if !strings.Contains(s, `"auth": "password"`) {
		t.Fatal("socks auth")
	}
}
