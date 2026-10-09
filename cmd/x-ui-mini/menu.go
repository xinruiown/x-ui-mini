package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func menu() {
	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(`
x-ui-mini 管理（输入 xui 也可打开）
  1) 查看状态
  2) 启动面板
  3) 停止面板
  4) 重启面板
  5) 改登录用户名
  6) 改登录密码
  7) 改面板端口
  8) 改面板路径
  9) 改公网 IP/域名
 10) 开启 BBR
 11) 查看最近日志
 12) 创建节点
 13) 列出节点
 14) 备份
 15) 更新（重新跑 install.sh）
 16) 卸载
  0) 退出
请选择: `)
		if !in.Scan() {
			return
		}
		switch strings.TrimSpace(in.Text()) {
		case "1":
			status()
			printBBR()
		case "2":
			runSys("systemctl", "start", "x-ui-mini")
		case "3":
			runSys("systemctl", "stop", "x-ui-mini")
		case "4":
			runSys("systemctl", "restart", "x-ui-mini")
		case "5":
			fmt.Print("新用户名: ")
			if in.Scan() {
				_ = os.Setenv("XUIMINI_USERNAME", strings.TrimSpace(in.Text()))
				must(configure())
				fmt.Println("已改用户名，正在重启")
				runSys("systemctl", "restart", "x-ui-mini")
			}
		case "6":
			fmt.Print("新密码: ")
			if in.Scan() {
				_ = os.Setenv("XUIMINI_PASSWORD", in.Text())
				must(configure())
				_ = os.Unsetenv("XUIMINI_PASSWORD")
				fmt.Println("已改密码，正在重启")
				runSys("systemctl", "restart", "x-ui-mini")
			}
		case "7":
			fmt.Print("新端口: ")
			if in.Scan() {
				_ = os.Setenv("XUIMINI_PORT", strings.TrimSpace(in.Text()))
				must(configure())
				fmt.Println("已改端口，正在重启")
				runSys("systemctl", "restart", "x-ui-mini")
			}
		case "8":
			fmt.Print("新路径（不要斜杠）: ")
			if in.Scan() {
				_ = os.Setenv("XUIMINI_PATH", strings.TrimSpace(in.Text()))
				must(configure())
				fmt.Println("已改路径，正在重启")
				runSys("systemctl", "restart", "x-ui-mini")
			}
		case "9":
			fmt.Print("公网 IP 或域名: ")
			if in.Scan() {
				must(setHost(strings.TrimSpace(in.Text())))
			}
		case "10":
			must(enableBBR())
		case "11":
			runSys("journalctl", "-u", "x-ui-mini", "-n", "40", "--no-pager")
		case "12":
			createNodeCLI(in)
		case "13":
			listNodes()
		case "14":
			must(backup(defaultBackupPath()))
		case "15":
			fmt.Println("curl -fsSL https://raw.githubusercontent.com/xinruiown/x-ui-mini/main/install.sh | sudo bash")
		case "16":
			fmt.Print("确认卸载？输入 yes: ")
			if in.Scan() && strings.TrimSpace(in.Text()) == "yes" {
				_ = os.Setenv("XUIMINI_UNINSTALL", "1")
				must(uninstall())
				return
			}
			fmt.Println("已取消")
		case "0", "q":
			return
		default:
			fmt.Println("无效选项")
		}
	}
}

func runSys(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("失败:", err)
	}
}

func enableBBR() error {
	if err := os.WriteFile("/etc/sysctl.d/99-x-ui-mini-bbr.conf", []byte("net.core.default_qdisc=fq\nnet.ipv4.tcp_congestion_control=bbr\n"), 0o644); err != nil {
		return err
	}
	_ = exec.Command("modprobe", "tcp_bbr").Run()
	if out, err := exec.Command("sysctl", "-p", "/etc/sysctl.d/99-x-ui-mini-bbr.conf").CombinedOutput(); err != nil {
		fmt.Print(string(out))
		return err
	}
	printBBR()
	fmt.Println("BBR 已写入 /etc/sysctl.d/99-x-ui-mini-bbr.conf")
	return nil
}

func printBBR() {
	cc, _ := os.ReadFile("/proc/sys/net/ipv4/tcp_congestion_control")
	av, _ := os.ReadFile("/proc/sys/net/ipv4/tcp_available_congestion_control")
	fmt.Printf("拥塞控制\t%s可用\t%s", cc, av)
}
