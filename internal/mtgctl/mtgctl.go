package mtgctl

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type Proc struct {
	mu  sync.Mutex
	cmd *exec.Cmd
	bin string
}

func New(bin string) *Proc { return &Proc{bin: bin} }

func DefaultBin() string {
	if p := os.Getenv("XUIMINI_MTG"); p != "" {
		return p
	}
	return "/usr/local/x-ui-mini/bin/mtg"
}

func (p *Proc) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil && p.cmd.Process != nil && p.cmd.ProcessState == nil
}

func (p *Proc) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
}

func (p *Proc) Restart(listen string, port int, secret string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopLocked()
	if secret == "" {
		return nil
	}
	if _, err := os.Stat(p.bin); err != nil {
		return fmt.Errorf("mtg 未安装: %s", p.bin)
	}
	bind := fmt.Sprintf("%s:%d", listen, port)
	cmd := exec.Command(p.bin, "simple-run", "-n", "1.1.1.1", bind, secret)
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

func (p *Proc) GenerateSecret(domain string) (string, error) {
	if domain == "" {
		domain = "www.cloudflare.com"
	}
	if _, err := os.Stat(p.bin); err != nil {
		return "", fmt.Errorf("mtg 未安装")
	}
	out, err := exec.Command(p.bin, "generate-secret", "--hex", domain).CombinedOutput()
	if err != nil {
		out, err = exec.Command(p.bin, "generate-secret", domain).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("generate-secret: %s %v", string(out), err)
		}
	}
	s := string(out)
	for _, c := range []string{"\n", "\r", " "} {
		if len(s) > 0 {
			// trim later
		}
		_ = c
	}
	b := make([]byte, 0, len(out))
	for _, ch := range out {
		if ch != ' ' && ch != '\n' && ch != '\r' && ch != '\t' {
			b = append(b, ch)
		}
	}
	if len(b) < 8 {
		return "", fmt.Errorf("secret 太短: %s", string(out))
	}
	return string(b), nil
}

func (p *Proc) stopLocked() {
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
	p.cmd = nil
}
