package sshx

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Target struct {
	Host     string
	Port     int
	Username string
	Password string
	// PrivateKey PEM optional; if set, preferred over password.
	PrivateKey string
}

func dial(t Target, timeout time.Duration) (*ssh.Client, error) {
	host := strings.TrimSpace(t.Host)
	if host == "" {
		return nil, fmt.Errorf("host empty")
	}
	port := t.Port
	if port <= 0 {
		port = 22
	}
	user := strings.TrimSpace(t.Username)
	if user == "" {
		user = "root"
	}
	var auths []ssh.AuthMethod
	if pem := strings.TrimSpace(t.PrivateKey); pem != "" {
		signer, err := ssh.ParsePrivateKey([]byte(pem))
		if err != nil {
			return nil, fmt.Errorf("parse key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if pw := t.Password; pw != "" {
		auths = append(auths, ssh.Password(pw))
	}
	if len(auths) == 0 {
		return nil, fmt.Errorf("no auth method")
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // internal ops tool; hosts managed by operators
		Timeout:         timeout,
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	return ssh.Dial("tcp", addr, cfg)
}

// TestConnect dials and runs a trivial command. timeout <= 0 defaults to 5s.
func TestConnect(t Target, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	cli, err := dial(t, timeout)
	if err != nil {
		return err
	}
	defer cli.Close()
	sess, err := cli.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	return sess.Run("true")
}

// RunScript executes script with a clean non-login bash (no profile/rc), optionally in workDir.
// Using bash -lc previously sourced login profiles, which can print unrelated paths or even
// exit before the deploy script runs — matching symptoms like a lone "/etc/..." line.
func RunScript(t Target, workDir, script string, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	cli, err := dial(t, 15*time.Second)
	if err != nil {
		return "", err
	}
	defer cli.Close()

	sess, err := cli.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()

	var buf bytes.Buffer
	sess.Stdout = &buf
	sess.Stderr = &buf

	cmd := strings.TrimSpace(script)
	if cmd == "" {
		return "", fmt.Errorf("script empty")
	}
	if wd := strings.TrimSpace(workDir); wd != "" {
		cmd = "cd " + shellQuote(wd) + " && " + cmd
	}
	// --noprofile --norc: do not load /etc/profile, ~/.bash_profile, ~/.bashrc, etc.
	remote := "bash --noprofile --norc -c " + shellQuote(cmd)
	done := make(chan error, 1)
	go func() {
		done <- sess.Run(remote)
	}()
	select {
	case err := <-done:
		return buf.String(), err
	case <-time.After(timeout):
		_ = sess.Close()
		return buf.String(), fmt.Errorf("timeout after %s", timeout)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
