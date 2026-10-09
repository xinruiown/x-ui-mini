package certs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func Dir() string {
	if p := os.Getenv("XUIMINI_DATA"); p != "" {
		return filepath.Join(p, "certs")
	}
	return "/usr/local/x-ui-mini/data/certs"
}

func EnsureSelfSigned(cn string) (cert, key string, err error) {
	if cn == "" {
		cn = "localhost"
	}
	dir := Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	cert = filepath.Join(dir, "server.crt")
	key = filepath.Join(dir, "server.key")
	if _, err := os.Stat(cert); err == nil {
		if _, err := os.Stat(key); err == nil {
			return cert, key, nil
		}
	}
	cmd := exec.Command("openssl", "req", "-x509", "-newkey", "rsa:2048", "-sha256", "-days", "3650", "-nodes",
		"-keyout", key, "-out", cert, "-subj", "/CN="+cn)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("openssl: %s %v", string(out), err)
	}
	return cert, key, nil
}

func IssueACME(domain, email string) (cert, key string, err error) {
	if domain == "" {
		return "", "", fmt.Errorf("需要域名")
	}
	if email == "" {
		email = "admin@" + domain
	}
	if _, err := exec.LookPath("certbot"); err != nil {
		return "", "", fmt.Errorf("未安装 certbot，请先 apt-get install -y certbot，或改用自签证书")
	}
	cmd := exec.Command("certbot", "certonly", "--standalone", "-d", domain, "--non-interactive", "--agree-tos", "-m", email, "--keep-until-expiring")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("certbot: %s %v", string(out), err)
	}
	live := filepath.Join("/etc/letsencrypt/live", domain)
	return filepath.Join(live, "fullchain.pem"), filepath.Join(live, "privkey.pem"), nil
}
