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

	"github.com/xinruiown/x-ui-mini/internal/keys"
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
  x-ui-mini              交互菜单（终端）
  x-ui-mini serve        启动面板与 Xray
  x-ui-mini status
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
	xr := xrayctl.New(xrayctl.DefaultBin(), filepath.Join(filepath.Dir(store.DefaultPath()), "xray.json"))
	cfg := st.Get()
	if raw, err := xraycfg.Build(cfg); err == nil {
		_ = xr.WriteConfig(raw)
		_ = xr.Restart()
	}
	srv := &web.Server{Store: st, Xray: xr}
	httpSrv := &http.Server{Addr: srv.ListenAddr(), Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		xr.Stop()
		_ = httpSrv.Close()
	}()
	fmt.Printf("x-ui-mini %s 监听 http://%s/%s （仅默认本机）\n", version.Version, srv.ListenAddr(), cfg.Panel.Path)
	err = httpSrv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
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

func menu() {
	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(`
x-ui-mini 菜单
1) 安装或初始化（serve 检测）
2) 创建节点
3) 节点管理
4) 任意门管理
5) Telegram 代理
6) 更新版本
7) 备份恢复
8) 服务状态
9) 卸载提示
0) 退出
请选择: `)
		if !in.Scan() {
			return
		}
		switch strings.TrimSpace(in.Text()) {
		case "1":
			fmt.Println("服务模式请使用: systemctl start x-ui-mini 或 x-ui-mini serve")
		case "2":
			createNodeCLI(in)
		case "3":
			listNodes()
		case "4":
			fmt.Println("任意门请用 WebUI 或后续 CLI；当前可用 Web 面板。")
		case "5":
			mtCLI(in)
		case "6":
			fmt.Println("curl -fsSL https://raw.githubusercontent.com/xinruiown/x-ui-mini/main/install.sh | sudo bash")
		case "7":
			fmt.Println("备份: x-ui-mini backup")
			fmt.Println("恢复: x-ui-mini restore <file>")
		case "8":
			status()
		case "9":
			fmt.Println("卸载: systemctl disable --now x-ui-mini && rm -rf /usr/local/x-ui-mini /usr/local/bin/x-ui-mini /etc/systemd/system/x-ui-mini.service")
		case "0", "q":
			return
		}
	}
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
