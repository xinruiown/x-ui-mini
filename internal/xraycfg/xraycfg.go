package xraycfg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/xinruiown/x-ui-mini/internal/store"
)

func Build(cfg store.Config) ([]byte, error) {
	inbounds := []any{}
	for _, n := range cfg.Nodes {
		if !n.Enabled {
			continue
		}
		switch n.Protocol {
		case "vless-reality":
			inbounds = append(inbounds, map[string]any{
				"tag":      "in-" + n.ID,
				"listen":   "0.0.0.0",
				"port":     n.Port,
				"protocol": "vless",
				"settings": map[string]any{
					"clients":    []any{map[string]any{"id": n.UUID, "flow": "xtls-rprx-vision"}},
					"decryption": "none",
				},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "reality",
					"realitySettings": map[string]any{
						"show":        false,
						"dest":        or(n.Reality.Dest, "www.cloudflare.com:443"),
						"xver":        0,
						"serverNames": orSlice(n.Reality.ServerNames, []string{"www.cloudflare.com"}),
						"privateKey":  n.Reality.PrivateKey,
						"shortIds":    []string{n.Reality.ShortID},
					},
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []string{"http", "tls", "quic"},
				},
			})
		case "shadowsocks":
			method := n.Method
			if method == "" {
				method = "aes-128-gcm"
			}
			inbounds = append(inbounds, map[string]any{
				"tag":      "in-" + n.ID,
				"listen":   "0.0.0.0",
				"port":     n.Port,
				"protocol": "shadowsocks",
				"settings": map[string]any{
					"method":   method,
					"password": n.Password,
					"network":  "tcp,udp",
				},
			})
		case "trojan":
			cert := or(cfg.Panel.CertFile, "/usr/local/x-ui-mini/data/certs/server.crt")
			key := or(cfg.Panel.KeyFile, "/usr/local/x-ui-mini/data/certs/server.key")
			inbounds = append(inbounds, map[string]any{
				"tag":      "in-" + n.ID,
				"listen":   "0.0.0.0",
				"port":     n.Port,
				"protocol": "trojan",
				"settings": map[string]any{
					"clients": []any{map[string]any{"password": n.Password}},
				},
				"streamSettings": map[string]any{
					"network":  "tcp",
					"security": "tls",
					"tlsSettings": map[string]any{
						"certificates": []any{map[string]any{"certificateFile": cert, "keyFile": key}},
					},
				},
			})
		}
	}
	if cfg.SOCKS != nil && cfg.SOCKS.Enabled {
		settings := map[string]any{"udp": cfg.SOCKS.UDP, "auth": "noauth"}
		if cfg.SOCKS.User != "" {
			settings["auth"] = "password"
			settings["accounts"] = []any{map[string]any{"user": cfg.SOCKS.User, "pass": cfg.SOCKS.Pass}}
		}
		inbounds = append(inbounds, map[string]any{
			"tag":      "socks-in",
			"listen":   or(cfg.SOCKS.Listen, "127.0.0.1"),
			"port":     cfg.SOCKS.Port,
			"protocol": "socks",
			"settings": settings,
		})
	}
	if cfg.HTTP != nil && cfg.HTTP.Enabled {
		settings := map[string]any{}
		if cfg.HTTP.User != "" {
			settings["accounts"] = []any{map[string]any{"user": cfg.HTTP.User, "pass": cfg.HTTP.Pass}}
		}
		inbounds = append(inbounds, map[string]any{
			"tag":      "http-in",
			"listen":   or(cfg.HTTP.Listen, "127.0.0.1"),
			"port":     cfg.HTTP.Port,
			"protocol": "http",
			"settings": settings,
		})
	}
	for _, f := range cfg.Forwards {
		if !f.Enabled {
			continue
		}
		netw := f.Network
		if netw == "" {
			netw = "tcp"
		}
		inbounds = append(inbounds, map[string]any{
			"tag":      "fwd-" + f.ID,
			"listen":   or(f.Listen, "0.0.0.0"),
			"port":     f.Port,
			"protocol": "dokodemo-door",
			"settings": map[string]any{
				"address":        strings.Split(f.Target, ":")[0],
				"port":           targetPort(f.Target),
				"network":        netw,
				"followRedirect": false,
			},
		})
	}

	doc := map[string]any{
		"log":       map[string]any{"loglevel": "warning"},
		"inbounds":  inbounds,
		"outbounds": []any{map[string]any{"protocol": "freedom", "tag": "direct"}, map[string]any{"protocol": "blackhole", "tag": "block"}},
	}
	return json.MarshalIndent(doc, "", "  ")
}

func Share(n store.Node, host string) string {
	if host == "" {
		host = "127.0.0.1"
	}
	name := url.QueryEscape(n.Name)
	switch n.Protocol {
	case "vless-reality":
		sni := "www.cloudflare.com"
		if len(n.Reality.ServerNames) > 0 {
			sni = n.Reality.ServerNames[0]
		}
		fp := n.Reality.Fingerprint
		if fp == "" {
			fp = "chrome"
		}
		return fmt.Sprintf("vless://%s@%s:%d?type=tcp&encryption=none&security=reality&pbk=%s&fp=%s&sni=%s&sid=%s&spx=%%2F&flow=xtls-rprx-vision#%s",
			n.UUID, host, n.Port, n.Reality.PublicKey, fp, sni, n.Reality.ShortID, name)
	case "shadowsocks":
		method := n.Method
		if method == "" {
			method = "aes-128-gcm"
		}
		userinfo := base64.StdEncoding.EncodeToString([]byte(method + ":" + n.Password))
		return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, host, n.Port, name)
	case "trojan":
		q := url.Values{}
		q.Set("security", "tls")
		q.Set("type", "tcp")
		q.Set("allowInsecure", "1")
		return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", n.Password, host, n.Port, q.Encode(), name)
	}
	return ""
}

func MTProtoLink(m store.MTProto, host string) string {
	if host == "" {
		host = "YOUR_IP"
	}
	return fmt.Sprintf("tg://proxy?server=%s&port=%d&secret=%s", host, m.Port, m.Secret)
}

func or(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func orSlice(v, d []string) []string {
	if len(v) == 0 {
		return d
	}
	return v
}

func targetPort(t string) int {
	parts := strings.Split(t, ":")
	if len(parts) < 2 {
		return 0
	}
	var p int
	_, _ = fmt.Sscanf(parts[len(parts)-1], "%d", &p)
	return p
}
