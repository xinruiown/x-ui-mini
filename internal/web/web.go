package web

import (
	"crypto/subtle"
	"encoding/json"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xinruiown/x-ui-mini/internal/certs"
	"github.com/xinruiown/x-ui-mini/internal/keys"
	"github.com/xinruiown/x-ui-mini/internal/mtgctl"
	"github.com/xinruiown/x-ui-mini/internal/store"
	"github.com/xinruiown/x-ui-mini/internal/xraycfg"
	"github.com/xinruiown/x-ui-mini/internal/xrayctl"
	"github.com/xinruiown/x-ui-mini/webui"
)

type Server struct {
	Store *store.Store
	Xray  *xrayctl.Proc
	Mtg   *mtgctl.Proc
	mu    sync.Mutex
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	static, _ := fs.Sub(webui.FS, ".")
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	mux.HandleFunc("/", s.ui)
	mux.HandleFunc("/api/login", s.login)
	mux.HandleFunc("/api/status", s.auth(s.status))
	mux.HandleFunc("/api/nodes", s.auth(s.nodes))
	mux.HandleFunc("/api/forwards", s.auth(s.forwards))
	mux.HandleFunc("/api/socks", s.auth(s.socks))
	mux.HandleFunc("/api/http", s.auth(s.httpIn))
	mux.HandleFunc("/api/mtproto", s.auth(s.mtproto))
	mux.HandleFunc("/api/apply", s.auth(s.apply))
	mux.HandleFunc("/api/settings", s.auth(s.settings))
	mux.HandleFunc("/api/cert", s.auth(s.cert))
	return mux
}

func (s *Server) ui(w http.ResponseWriter, r *http.Request) {
	cfg := s.Store.Get()
	prefix := "/" + cfg.Panel.Path
	if r.URL.Path != prefix && r.URL.Path != prefix+"/" {
		http.NotFound(w, r)
		return
	}
	b, _ := fs.ReadFile(webui.FS, "index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var in struct{ Username, Password string }
	_ = json.NewDecoder(r.Body).Decode(&in)
	cfg := s.Store.Get()
	okUser := subtle.ConstantTimeCompare([]byte(in.Username), []byte(cfg.Panel.Username)) == 1
	okPass := subtle.ConstantTimeCompare([]byte(store.HashPassword(cfg.Panel.PasswordSalt, in.Password)), []byte(cfg.Panel.PasswordHash)) == 1
	if !okUser || !okPass {
		http.Error(w, "unauthorized", 401)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "xui", Value: sessionToken(cfg), Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	writeJSON(w, map[string]any{"ok": true})
}

func sessionToken(cfg store.Config) string {
	return store.HashPassword(cfg.Panel.PasswordSalt, cfg.Panel.PasswordHash+cfg.Panel.Path)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("xui")
		cfg := s.Store.Get()
		if err != nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(sessionToken(cfg))) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		next(w, r)
	}
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	cfg := s.Store.Get()
	writeJSON(w, map[string]any{
		"listen":     cfg.Panel.Listen + ":" + strconv.Itoa(cfg.Panel.Port),
		"path":       cfg.Panel.Path,
		"xray":       s.Xray.Running(),
		"nodes":      len(cfg.Nodes),
		"forwards":   len(cfg.Forwards),
		"socks":      cfg.SOCKS,
		"http":       cfg.HTTP,
		"mtproto":    cfg.MTProto != nil && cfg.MTProto.Enabled,
		"mtg":        s.Mtg != nil && s.Mtg.Running(),
		"publicHost": cfg.Panel.PublicHost,
		"domain":     cfg.Panel.Domain,
		"cert_file":  cfg.Panel.CertFile,
	})
}

func (s *Server) nodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := s.Store.Get()
		type out struct {
			store.Node
			Share string `json:"share"`
		}
		list := []out{}
		for _, n := range cfg.Nodes {
			list = append(list, out{Node: n, Share: xraycfg.Share(n, cfg.Panel.PublicHost)})
		}
		writeJSON(w, list)
	case http.MethodPost:
		var in store.Node
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if err := s.createNode(in); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		_ = s.reload()
		cfg := s.Store.Get()
		var created store.Node
		if len(cfg.Nodes) > 0 {
			created = cfg.Nodes[len(cfg.Nodes)-1]
		}
		writeJSON(w, map[string]any{"ok": true, "share": xraycfg.Share(created, cfg.Panel.PublicHost)})
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		var in struct {
			Name string `json:"name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if id == "" || in.Name == "" {
			http.Error(w, "需要 id 和 name", 400)
			return
		}
		err := s.Store.Update(func(c *store.Config) error {
			for i := range c.Nodes {
				if c.Nodes[i].ID == id {
					c.Nodes[i].Name = in.Name
					return nil
				}
			}
			return errf("节点不存在")
		})
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		_ = s.Store.Update(func(c *store.Config) error {
			ns := c.Nodes[:0]
			for _, n := range c.Nodes {
				if n.ID != id {
					ns = append(ns, n)
				}
			}
			c.Nodes = ns
			return nil
		})
		_ = s.reload()
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "method", 405)
	}
}

func (s *Server) createNode(in store.Node) error {
	if in.Port < 1 || in.Port > 65535 {
		return errf("端口无效")
	}
	in.ID = store.NewID()
	in.Enabled = true
	if in.Name == "" {
		in.Name = in.Protocol + "-" + strconv.Itoa(in.Port)
	}
	switch in.Protocol {
	case "vless-reality":
		in.UUID = keys.UUID()
		priv, pub, err := keys.RealityKeyPair()
		if err != nil {
			return err
		}
		in.Reality.PrivateKey = priv
		in.Reality.PublicKey = pub
		in.Reality.ShortID = keys.ShortID()
		if in.Reality.Dest == "" {
			in.Reality.Dest = "www.cloudflare.com:443"
		}
		if len(in.Reality.ServerNames) == 0 {
			in.Reality.ServerNames = []string{"www.cloudflare.com"}
		}
		if in.Reality.Fingerprint == "" {
			in.Reality.Fingerprint = "chrome"
		}
	case "shadowsocks":
		if in.Method == "" {
			in.Method = "aes-128-gcm"
		}
		in.Password = keys.Password(16)
	case "trojan":
		in.Password = keys.Password(16)
	default:
		return errf("未知协议")
	}
	return s.Store.Update(func(c *store.Config) error {
		c.Nodes = append(c.Nodes, in)
		return nil
	})
}

func (s *Server) forwards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.Store.Get().Forwards)
	case http.MethodPost:
		var in store.Forward
		_ = json.NewDecoder(r.Body).Decode(&in)
		in.ID = store.NewID()
		in.Enabled = true
		if in.Mode == "" {
			in.Mode = "relay"
		}
		if in.Listen == "" {
			in.Listen = "0.0.0.0"
		}
		if in.Network == "" {
			in.Network = "tcp"
		}
		_ = s.Store.Update(func(c *store.Config) error {
			c.Forwards = append(c.Forwards, in)
			return nil
		})
		_ = s.reload()
		writeJSON(w, map[string]any{"ok": true})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		_ = s.Store.Update(func(c *store.Config) error {
			fs := c.Forwards[:0]
			for _, f := range c.Forwards {
				if f.ID != id {
					fs = append(fs, f)
				}
			}
			c.Forwards = fs
			return nil
		})
		_ = s.reload()
		writeJSON(w, map[string]any{"ok": true})
	}
}

func (s *Server) socks(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var in store.SOCKS
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Listen == "" {
			in.Listen = "127.0.0.1"
		}
		if in.Port == 0 {
			in.Port = 1080
		}
		_ = s.Store.Update(func(c *store.Config) error { c.SOCKS = &in; return nil })
		_ = s.reload()
	}
	writeJSON(w, s.Store.Get().SOCKS)
}

func (s *Server) httpIn(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var in store.HTTP
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Listen == "" {
			in.Listen = "127.0.0.1"
		}
		if in.Port == 0 {
			in.Port = 8080
		}
		_ = s.Store.Update(func(c *store.Config) error { c.HTTP = &in; return nil })
		_ = s.reload()
	}
	writeJSON(w, s.Store.Get().HTTP)
}

func (s *Server) mtproto(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var in store.MTProto
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Listen == "" {
			in.Listen = "0.0.0.0"
		}
		if in.Port == 0 {
			in.Port = 4430
		}
		if s.Mtg == nil {
			s.Mtg = mtgctl.New(mtgctl.DefaultBin())
		}
		if in.Secret == "" {
			sec, err := s.Mtg.GenerateSecret("www.cloudflare.com")
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			in.Secret = sec
		}
		_ = s.Store.Update(func(c *store.Config) error { c.MTProto = &in; return nil })
		if in.Enabled {
			if err := s.Mtg.Restart(in.Listen, in.Port, in.Secret); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		} else {
			s.Mtg.Stop()
		}
	}
	cfg := s.Store.Get()
	if cfg.MTProto == nil {
		writeJSON(w, map[string]any{"enabled": false})
		return
	}
	writeJSON(w, map[string]any{
		"config":  cfg.MTProto,
		"link":    xraycfg.MTProtoLink(*cfg.MTProto, cfg.Panel.PublicHost),
		"running": s.Mtg != nil && s.Mtg.Running(),
	})
}

func (s *Server) cert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		cfg := s.Store.Get()
		writeJSON(w, map[string]any{"domain": cfg.Panel.Domain, "cert_file": cfg.Panel.CertFile, "key_file": cfg.Panel.KeyFile})
		return
	}
	var in struct {
		Domain string `json:"domain"`
		Email  string `json:"email"`
		Mode   string `json:"mode"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	var cert, key string
	var err error
	switch in.Mode {
	case "acme":
		cert, key, err = certs.IssueACME(in.Domain, in.Email)
	default:
		cn := in.Domain
		if cn == "" {
			cn = "localhost"
		}
		cert, key, err = certs.EnsureSelfSigned(cn)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	_ = s.Store.Update(func(c *store.Config) error {
		if in.Domain != "" {
			c.Panel.Domain = in.Domain
			c.Panel.PublicHost = in.Domain
		}
		c.Panel.CertFile, c.Panel.KeyFile = cert, key
		return nil
	})
	writeJSON(w, map[string]any{"ok": true, "cert_file": cert, "key_file": key})
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := s.Store.Get()
		writeJSON(w, map[string]any{
			"listen":      cfg.Panel.Listen,
			"port":        cfg.Panel.Port,
			"path":        cfg.Panel.Path,
			"username":    cfg.Panel.Username,
			"public_host": cfg.Panel.PublicHost,
			"domain":      cfg.Panel.Domain,
		})
	case http.MethodPost:
		var in struct {
			PublicHost string `json:"public_host"`
			Username   string `json:"username"`
			Password   string `json:"password"`
			Path       string `json:"path"`
			Listen     string `json:"listen"`
			Port       int    `json:"port"`
			Domain     string `json:"domain"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		err := s.Store.Update(func(c *store.Config) error {
			if in.PublicHost != "" {
				c.Panel.PublicHost = in.PublicHost
			}
			if in.Username != "" {
				c.Panel.Username = in.Username
			}
			if in.Password != "" {
				c.Panel.PasswordSalt = store.RandHex(8)
				c.Panel.PasswordHash = store.HashPassword(c.Panel.PasswordSalt, in.Password)
			}
			if in.Path != "" {
				c.Panel.Path = strings.Trim(in.Path, "/")
			}
			if in.Listen != "" {
				c.Panel.Listen = in.Listen
			}
			if in.Port > 0 {
				c.Panel.Port = in.Port
			}
			if in.Domain != "" {
				c.Panel.Domain = in.Domain
				c.Panel.PublicHost = in.Domain
			}
			return nil
		})
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	default:
		http.Error(w, "method", 405)
	}
}

func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	if err := s.reload(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.Store.Get()
	raw, err := xraycfg.Build(cfg)
	if err != nil {
		return err
	}
	if err := s.Xray.WriteConfig(raw); err != nil {
		return err
	}
	if err := s.Xray.Restart(); err != nil {
		return err
	}
	if s.Mtg != nil && cfg.MTProto != nil && cfg.MTProto.Enabled {
		_ = s.Mtg.Restart(orListen(cfg.MTProto.Listen, "0.0.0.0"), cfg.MTProto.Port, cfg.MTProto.Secret)
	}
	return nil
}

func orListen(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func (s *Server) ListenAddr() string {
	cfg := s.Store.Get()
	return net.JoinHostPort(cfg.Panel.Listen, strconv.Itoa(cfg.Panel.Port))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

type constErr string

func (e constErr) Error() string { return string(e) }
func errf(s string) error        { return constErr(s) }

func init() { _ = time.Now; _ = strings.TrimSpace }
