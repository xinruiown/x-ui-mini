package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/xinruiown/x-ui-mini/internal/certs"
	"github.com/xinruiown/x-ui-mini/internal/keys"
	"github.com/xinruiown/x-ui-mini/internal/mtgctl"
	"github.com/xinruiown/x-ui-mini/internal/store"
	"github.com/xinruiown/x-ui-mini/internal/version"
	"github.com/xinruiown/x-ui-mini/internal/web"
	"github.com/xinruiown/x-ui-mini/internal/xraycfg"
	"github.com/xinruiown/x-ui-mini/internal/xrayctl"
)

func main() {
	if len(os.Args) < 2 {
		if isTTY() {
			menu()
			return
		}
		usage()
		return
	}
	switch os.Args[1] {
	case "serve":
		must(serve())
	case "set-host":
		must(setHost(arg(2, "")))
	case "configure":
		must(configure())
	case "uninstall":
		must(uninstall())
	case "bbr":
		must(enableBBR())
	case "status":
		status()
	case "backup":
		must(backup(arg(2, defaultBackupPath())))
	case "restore":
		must(restore(arg(2, "")))
	case "update":
		fmt.Println("请使用: curl -fsSL https://raw.githubusercontent.com/xinruiown/x-ui-mini/main/install.sh | sudo bash")
	case "version", "-v", "--version":
		fmt.Println("x-ui-mini", version.Version, version.Commit)
	case "menu":
		menu()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`x-ui-mini — 超轻量中文节点面板

用法:
  xui / x-ui-mini     交互菜单
  x-ui-mini serve
  x-ui-mini status
  x-ui-mini bbr
  x-ui-mini backup [file]
  x-ui-mini restore <file>
  x-ui-mini update
  x-ui-mini version
`)
}

func serve() error {
	st, err := store.Open(store.DefaultPath())
	if err != nil {
		return err
	}
	ensureBootstrap(st)
	if h := detectPublicHost(); h != "" {
		_ = st.Update(func(c *store.Config) error {
			if c.Panel.PublicHost == "" {
				c.Panel.PublicHost = h
			}
			if c.Panel.Listen == "127.0.0.1" {
				c.Panel.Listen = "0.0.0.0"
			}
			return nil
		})
	}
	xr := xrayctl.New(xrayctl.DefaultBin(), filepath.Join(filepath.Dir(store.DefaultPath()), "xray.json"))
	mtg := mtgctl.New(mtgctl.DefaultBin())
	if _, _, err := certs.EnsureSelfSigned("localhost"); err == nil {
		_ = st.Update(func(c *store.Config) error {
			if c.Panel.CertFile == "" {
				c.Panel.CertFile = filepath.Join(filepath.Dir(store.DefaultPath()), "certs", "server.crt")
				c.Panel.KeyFile = filepath.Join(filepath.Dir(store.DefaultPath()), "certs", "server.key")
			}
			return nil
		})
	}
	cfg := st.Get()
	if raw, err := xraycfg.Build(cfg); err == nil {
		_ = xr.WriteConfig(raw)
		_ = xr.Restart()
	}
	if cfg.MTProto != nil && cfg.MTProto.Enabled {
		_ = mtg.Restart(cfg.MTProto.Listen, cfg.MTProto.Port, cfg.MTProto.Secret)
	}
	srv := &web.Server{Store: st, Xray: xr, Mtg: mtg}
	httpSrv := &http.Server{Addr: srv.ListenAddr(), Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		xr.Stop()
		mtg.Stop()
		_ = httpSrv.Close()
	}()
	fmt.Printf("x-ui-mini %s 监听 http://%s/%s\n", version.Version, srv.ListenAddr(), cfg.Panel.Path)
	err = httpSrv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func setHost(host string) error {
	if host == "" {
		host = detectPublicHost()
	}
	if host == "" {
		return fmt.Errorf("无法检测公网地址，请执行: x-ui-mini set-host 你的IP或域名")
	}
	st, err := store.Open(store.DefaultPath())
	if err != nil {
		return err
	}
	return st.Update(func(c *store.Config) error {
		c.Panel.PublicHost = host
		return nil
	})
}

func detectPublicHost() string {
	if v := strings.TrimSpace(os.Getenv("XUIMINI_HOST")); v != "" {
		return v
	}
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	h := strings.TrimSpace(string(b))
	if h == "" || strings.ContainsAny(h, " \n<>") {
		return ""
	}
	return h
}

func configure() error {
	st, err := store.Open(store.DefaultPath())
	if err != nil {
		return err
	}
	return st.Update(func(c *store.Config) error {
		if v := strings.TrimSpace(os.Getenv("XUIMINI_USERNAME")); v != "" {
			c.Panel.Username = v
		}
		if v := os.Getenv("XUIMINI_PASSWORD"); v != "" {
			c.Panel.PasswordSalt = store.RandHex(8)
			c.Panel.PasswordHash = store.HashPassword(c.Panel.PasswordSalt, v)
			_ = os.WriteFile(filepath.Join(filepath.Dir(st.Path()), "initial-password.txt"), []byte(v+"\n"), 0o600)
		}
		if v := strings.Trim(strings.TrimSpace(os.Getenv("XUIMINI_PATH")), "/"); v != "" {
			c.Panel.Path = v
		}
		if v := strings.TrimSpace(os.Getenv("XUIMINI_HOST")); v != "" {
			c.Panel.PublicHost = v
		}
		if v := strings.TrimSpace(os.Getenv("XUIMINI_LISTEN")); v != "" {
			c.Panel.Listen = v
		} else {
			c.Panel.Listen = "0.0.0.0"
		}
		if v := strings.TrimSpace(os.Getenv("XUIMINI_PORT")); v != "" {
			if p, err := strconv.Atoi(v); err == nil && p > 0 {
				c.Panel.Port = p
			}
		}
		if v := strings.TrimSpace(os.Getenv("XUIMINI_DOMAIN")); v != "" {
			c.Panel.Domain = v
			if c.Panel.PublicHost == "" {
				c.Panel.PublicHost = v
			}
		}
		return nil
	})
}

func uninstall() error {
	if os.Getenv("XUIMINI_UNINSTALL") != "1" {
		return fmt.Errorf("确认卸载请设置 XUIMINI_UNINSTALL=1")
	}
	_ = exec.Command("systemctl", "disable", "--now", "x-ui-mini").Run()
	_ = os.Remove("/etc/systemd/system/x-ui-mini.service")
	_ = os.Remove("/usr/local/bin/x-ui-mini")
	_ = os.Remove("/usr/local/bin/xui")
	_ = os.RemoveAll("/usr/local/x-ui-mini")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	fmt.Println("已卸载 x-ui-mini")
	return nil
}

func ensureBootstrap(st *store.Store) {
	p := filepath.Join(filepath.Dir(st.Path()), "initial-password.txt")
	if _, err := os.Stat(p); err == nil {
		return
	}
	cfg := st.Get()
	if cfg.Panel.PasswordHash == "" {
		return
	}
	// If password file missing after first run, do not rotate. Only write when brand new dir empty besides config.
}

func status() {
	st, err := store.Open(store.DefaultPath())
	if err != nil {
		fmt.Println("未初始化:", err)
		os.Exit(1)
	}
	cfg := st.Get()
	fmt.Printf("version\t%s\nlisten\t%s:%d\npath\t/%s\nnodes\t%d\nforwards\t%d\n", version.Version, cfg.Panel.Listen, cfg.Panel.Port, cfg.Panel.Path, len(cfg.Nodes), len(cfg.Forwards))
	if _, err := os.Stat("/usr/local/x-ui-mini/bin/xray"); err == nil {
		fmt.Println("xray\tinstalled")
	} else {
		fmt.Println("xray\tmissing")
	}
}

func defaultBackupPath() string {
	return fmt.Sprintf("/usr/local/x-ui-mini/backup/x-ui-mini-%s.tar.gz", time.Now().Format("20060102-150405"))
}

func backup(out string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o700); err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	root := filepath.Dir(store.DefaultPath())
	absOut, _ := filepath.Abs(out)
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		abs, _ := filepath.Abs(path)
		if abs == absOut {
			return nil
		}
		if strings.Contains(path, "/backup/") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		hdr, _ := tar.FileInfoHeader(info, "")
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		rf, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, rf)
		rf.Close()
		return err
	})
}

func restore(in string) error {
	if in == "" {
		return fmt.Errorf("需要备份文件路径")
	}
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	root := filepath.Dir(store.DefaultPath())
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		dest := filepath.Join(root, hdr.Name)
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			return err
		}
		of, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
		if err != nil {
			return err
		}
		_, err = io.Copy(of, tr)
		of.Close()
		if err != nil {
			return err
		}
	}
	fmt.Println("已恢复，请执行: systemctl restart x-ui-mini")
	return nil
}

func createNodeCLI(in *bufio.Scanner) {
	st, err := store.Open(store.DefaultPath())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print("协议 [vless-reality/shadowsocks/trojan]: ")
	in.Scan()
	proto := strings.TrimSpace(in.Text())
	fmt.Print("端口: ")
	in.Scan()
	port, _ := strconv.Atoi(strings.TrimSpace(in.Text()))
	n := store.Node{ID: store.NewID(), Protocol: proto, Port: port, Enabled: true, Name: proto}
	switch proto {
	case "vless-reality":
		n.UUID = keys.UUID()
		priv, pub, err := keys.RealityKeyPair()
		if err != nil {
			fmt.Println(err)
			return
		}
		n.Reality.PrivateKey, n.Reality.PublicKey, n.Reality.ShortID = priv, pub, keys.ShortID()
		n.Reality.Dest = "www.cloudflare.com:443"
		n.Reality.ServerNames = []string{"www.cloudflare.com"}
		n.Reality.Fingerprint = "chrome"
	case "shadowsocks":
		n.Method, n.Password = "aes-128-gcm", keys.Password(16)
	case "trojan":
		n.Password = keys.Password(16)
	default:
		fmt.Println("未知协议")
		return
	}
	_ = st.Update(func(c *store.Config) error { c.Nodes = append(c.Nodes, n); return nil })
	cfg := st.Get()
	fmt.Println(xraycfg.Share(n, cfg.Panel.PublicHost))
}

func listNodes() {
	st, err := store.Open(store.DefaultPath())
	if err != nil {
		fmt.Println(err)
		return
	}
	cfg := st.Get()
	for _, n := range cfg.Nodes {
		fmt.Printf("%s  %s  :%d\n%s\n", n.ID, n.Protocol, n.Port, xraycfg.Share(n, cfg.Panel.PublicHost))
	}
}

func mtCLI(in *bufio.Scanner) {
	st, err := store.Open(store.DefaultPath())
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print("端口: ")
	in.Scan()
	port, _ := strconv.Atoi(strings.TrimSpace(in.Text()))
	m := store.MTProto{Listen: "0.0.0.0", Port: port, Secret: keys.MTProtoSecret(), Enabled: true}
	_ = st.Update(func(c *store.Config) error { c.MTProto = &m; return nil })
	fmt.Println(xraycfg.MTProtoLink(m, st.Get().Panel.PublicHost))
}

func isTTY() bool {
	st, _ := os.Stdout.Stat()
	return st.Mode()&os.ModeCharDevice != 0
}

func arg(i int, def string) string {
	if len(os.Args) > i {
		return os.Args[i]
	}
	return def
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() { _ = exec.ErrNotFound }
