// Package ssh provides a portable SSH script runner.
// Implementation currently delegates to internal/sshx; logic can move here later
// without changing the public Config/Ping/Run surface.
package ssh

import (
	"context"
	"fmt"
	"strings"
	"time"

	"depman/internal/connector"
	"depman/internal/sshx"
)

// Config is SSH connection parameters (no env/project awareness).
type Config struct {
	Host       string
	Port       int
	Username   string
	Password   string
	PrivateKey string // PEM; preferred over Password when set
	Timeout    time.Duration
}

func (c Config) validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("%w: host empty", connector.ErrInvalidConfig)
	}
	if c.PrivateKey == "" && c.Password == "" {
		return fmt.Errorf("%w: no auth method", connector.ErrInvalidConfig)
	}
	return nil
}

func (c Config) target() sshx.Target {
	return sshx.Target{
		Host:       c.Host,
		Port:       c.Port,
		Username:   c.Username,
		Password:   c.Password,
		PrivateKey: c.PrivateKey,
	}
}

// Client is a reusable SSH connector.
type Client struct {
	Cfg Config
}

func New(cfg Config) *Client { return &Client{Cfg: cfg} }

func (c *Client) Ping(ctx context.Context) error {
	if err := c.Cfg.validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	timeout := c.Cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if dl, ok := ctx.Deadline(); ok {
		if rem := time.Until(dl); rem > 0 && rem < timeout {
			timeout = rem
		}
	}
	return sshx.TestConnect(c.Cfg.target(), timeout)
}

// Run executes script via clean bash --noprofile --norc -c, optionally after cd into workDir.
func (c *Client) Run(ctx context.Context, workDir, script string, timeout time.Duration) (sshx.ScriptResult, error) {
	var empty sshx.ScriptResult
	if err := c.Cfg.validate(); err != nil {
		return empty, err
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if timeout <= 0 {
		timeout = c.Cfg.Timeout
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	return sshx.RunScript(c.Cfg.target(), workDir, script, timeout)
}

// ConfigFromSecret fills Password or PrivateKey from a raw secret string.
// Empty secret leaves both unset so validate() fails with a clear error.
func ConfigFromSecret(host string, port int, username, secret string) Config {
	cfg := Config{Host: host, Port: port, Username: username}
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return cfg
	}
	if strings.Contains(secret, "BEGIN") && strings.Contains(secret, "PRIVATE KEY") {
		cfg.PrivateKey = secret
	} else {
		cfg.Password = secret
	}
	return cfg
}
