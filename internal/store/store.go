package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Config struct {
	Version  string    `json:"version"`
	Panel    Panel     `json:"panel"`
	Nodes    []Node    `json:"nodes"`
	Forwards []Forward `json:"forwards"`
	SOCKS    *SOCKS    `json:"socks,omitempty"`
	MTProto  *MTProto  `json:"mtproto,omitempty"`
	Modules  Modules   `json:"modules"`
}

type Panel struct {
	Listen       string `json:"listen"`
	Port         int    `json:"port"`
	Username     string `json:"username"`
	PasswordSalt string `json:"password_salt"`
	PasswordHash string `json:"password_hash"`
	Path         string `json:"path"`
	PublicHost   string `json:"public_host"`
}

type Node struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Protocol string  `json:"protocol"` // vless-reality, shadowsocks, trojan
	Port     int     `json:"port"`
	UUID     string  `json:"uuid,omitempty"`
	Password string  `json:"password,omitempty"`
	Method   string  `json:"method,omitempty"`
	Reality  Reality `json:"reality,omitempty"`
	Enabled  bool    `json:"enabled"`
}

type Reality struct {
	PrivateKey   string   `json:"private_key"`
	PublicKey    string   `json:"public_key"`
	ShortID      string   `json:"short_id"`
	ServerNames  []string `json:"server_names"`
	Dest         string   `json:"dest"`
	Fingerprint  string   `json:"fingerprint"`
}

type Forward struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Mode     string `json:"mode"` // relay, ingress, egress
	Listen   string `json:"listen"`
	Port     int    `json:"port"`
	Network  string `json:"network"` // tcp, udp, tcp,udp
	Target   string `json:"target"`
	Enabled  bool   `json:"enabled"`
}

type SOCKS struct {
	Listen   string `json:"listen"`
	Port     int    `json:"port"`
	User     string `json:"user,omitempty"`
	Pass     string `json:"pass,omitempty"`
	UDP      bool   `json:"udp"`
	Enabled  bool   `json:"enabled"`
}

type MTProto struct {
	Listen  string `json:"listen"`
	Port    int    `json:"port"`
	Secret  string `json:"secret"`
	Enabled bool   `json:"enabled"`
}

type Modules struct {
	AmneziaWG bool `json:"amneziawg"`
	Hysteria2 bool `json:"hysteria2"`
}

type Store struct {
	mu   sync.Mutex
	path string
	cfg  Config
}

func DefaultPath() string {
	if p := os.Getenv("XUIMINI_DATA"); p != "" {
		return filepath.Join(p, "config.json")
	}
	return "/usr/local/x-ui-mini/data/config.json"
}

func Open(path string) (*Store, error) {
	s := &Store{path: path}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		pass := randHex(8)
		s.cfg = defaultConfig(pass)
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
		_ = os.WriteFile(filepath.Join(filepath.Dir(path), "initial-password.txt"), []byte(pass+"\n"), 0o600)
		return s, nil
	}
	if err := json.Unmarshal(b, &s.cfg); err != nil {
		return nil, err
	}
	return s, nil
}

func defaultConfig(pass string) Config {
	salt := randHex(8)
	return Config{
		Version: "1",
		Panel: Panel{
			Listen:       "127.0.0.1",
			Port:         2053,
			Username:     "admin",
			PasswordSalt: salt,
			PasswordHash: HashPassword(salt, pass),
			Path:         randHex(8),
			PublicHost:   "",
		},
		Nodes:    []Node{},
		Forwards: []Forward{},
		SOCKS: &SOCKS{
			Listen:  "127.0.0.1",
			Port:    1080,
			Enabled: false,
			UDP:     false,
		},
		Modules: Modules{},
	}
}

func HashPassword(salt, password string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(sum[:])
}

func (s *Store) Path() string { return s.path }

func (s *Store) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return clone(s.cfg)
}

func (s *Store) Update(fn func(*Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := clone(s.cfg)
	if err := fn(&cfg); err != nil {
		return err
	}
	s.cfg = cfg
	return s.saveLocked()
}

func (s *Store) BootstrapPassword() (string, error) {
	// Only returned when file was just created: password is not stored in clear.
	// Caller should print from env XUIMINI_BOOTSTRAP if set.
	return os.Getenv("XUIMINI_BOOTSTRAP"), nil
}

func (s *Store) saveLocked() error {
	s.cfg.Version = "1"
	b, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func clone(c Config) Config {
	b, _ := json.Marshal(c)
	var out Config
	_ = json.Unmarshal(b, &out)
	return out
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func NewID() string {
	return randHex(8) + hex.EncodeToString([]byte(time.Now().Format("150405")))[:4]
}

func RandHex(n int) string { return randHex(n) }
