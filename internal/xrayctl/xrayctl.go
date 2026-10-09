package xrayctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
)

type Proc struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	bin     string
	cfgPath string
}

func New(bin, cfgPath string) *Proc {
	return &Proc{bin: bin, cfgPath: cfgPath}
}

func DefaultBin() string {
	if p := os.Getenv("XUIMINI_XRAY"); p != "" {
		return p
	}
	return "/usr/local/x-ui-mini/bin/xray"
}

func (p *Proc) WriteConfig(raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(p.cfgPath), 0o700); err != nil {
		return err
	}
	tmp := p.cfgPath + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.cfgPath)
}

func (p *Proc) Restart() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	if _, err := os.Stat(p.bin); err != nil {
		return fmt.Errorf("xray 二进制不存在: %s（安装时会下载 Xray-core）", p.bin)
	}
	cmd := exec.Command(p.bin, "run", "-c", p.cfgPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	p.cmd = cmd
	go func() { _ = cmd.Wait() }()
	return nil
}

func (p *Proc) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

func (p *Proc) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil && p.cmd.Process != nil
}

func (p *Proc) stopLocked() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
	p.cmd = nil
}
