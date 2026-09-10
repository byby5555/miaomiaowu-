// Package singboxops provides high-level operations for managing a remote
// sing-box server over SSH.
package singboxops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"miaomiaowu/internal/sshclient"
)

// Remote is the information needed to operate a sing-box server.
type Remote struct {
	Host        string
	Port        int
	SSHUser     string
	AuthType    string // "password" | "private_key"
	AuthData    string
	ConfigPath  string
	APIPort     int
	SingboxPort int
	SingboxUser string
}

// Status describes a remote sing-box server.
type Status struct {
	SSHOK        bool   `json:"ssh_ok"`
	Running      bool   `json:"running"`
	Version      string `json:"version"`
	ConfigExists bool   `json:"config_exists"`
	Message      string `json:"message"`
}

// Client wraps sshclient for sing-box operations.
type Client struct {
	ssh *sshclient.Client
	cfg Remote
}

// New creates a sing-box ops client.
func New(cfg Remote) (*Client, error) {
	if cfg.Host == "" {
		return nil, errors.New("remote host is required")
	}
	if cfg.Port <= 0 {
		cfg.Port = 22
	}
	if cfg.SSHUser == "" {
		cfg.SSHUser = "root"
	}
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = "/etc/sing-box/config.json"
	}
	if cfg.APIPort <= 0 {
		cfg.APIPort = 9090
	}
	if cfg.SingboxPort <= 0 {
		cfg.SingboxPort = 7890
	}

	sshOpts := sshclient.Options{
		Host: cfg.Host,
		Port: cfg.Port,
		User: cfg.SSHUser,
	}
	if cfg.AuthType == "private_key" {
		sshOpts.PrivateKey = cfg.AuthData
	} else {
		sshOpts.Password = cfg.AuthData
	}

	client, err := sshclient.New(sshOpts)
	if err != nil {
		return nil, err
	}
	return &Client{ssh: client, cfg: cfg}, nil
}

// TestConnection 执行 SSH 连通性测试。
func (c *Client) TestConnection(ctx context.Context) error {
	_, err := c.ssh.Run(ctx, "echo ok")
	return err
}

// GetStatus 查询 sing-box 运行状态与版本。
func (c *Client) GetStatus(ctx context.Context) (Status, error) {
	var st Status
	if err := c.TestConnection(ctx); err != nil {
		st.Message = "SSH 连接失败: " + err.Error()
		return st, nil
	}
	st.SSHOK = true

	if out, err := c.ssh.Run(ctx, "sing-box version 2>&1 | head -1"); err == nil {
		st.Version = strings.TrimSpace(out)
	}
	if out, err := c.ssh.Run(ctx, "systemctl is-active sing-box 2>/dev/null || pgrep -x sing-box >/dev/null && echo running || echo stopped"); err == nil {
		st.Running = strings.Contains(out, "running") || strings.Contains(out, "active")
	}
	if out, err := c.ssh.Run(ctx, fmt.Sprintf("test -f %s && echo yes || echo no", shellQuote(c.cfg.ConfigPath))); err == nil {
		st.ConfigExists = strings.Contains(out, "yes")
	}
	st.Message = "OK"
	return st, nil
}

// GenerateConfig 生成 sing-box 配置 JSON。
func (c *Client) GenerateConfig() ([]byte, error) {
	cfg := buildConfig(c.cfg)
	return json.MarshalIndent(cfg, "", "  ")
}

// buildConfig 构造基础 sing-box 配置。
func buildConfig(cfg Remote) map[string]any {
	outbounds := []map[string]any{
		{"type": "direct", "tag": "direct"},
	}
	if cfg.SingboxUser != "" {
		outbounds = append(outbounds, map[string]any{
			"type":        "socks",
			"tag":         "upstream",
			"server":      "127.0.0.1",
			"server_port": 1080,
			"username":    cfg.SingboxUser,
		})
	}
	inbound := map[string]any{
		"type":        "mixed",
		"tag":         "mixed-in",
		"listen":      "0.0.0.0",
		"listen_port": cfg.SingboxPort,
	}
	if cfg.SingboxUser != "" {
		inbound["users"] = []map[string]any{{"username": cfg.SingboxUser, "password": cfg.SingboxUser}}
	}
	return map[string]any{
		"log":      map[string]any{"level": "info"},
		"inbounds": []map[string]any{inbound},
		"outbounds": outbounds,
		"route": map[string]any{
			"rules": []map[string]any{},
		},
		"experimental": map[string]any{
			"clash_api": map[string]any{
				"external_controller": fmt.Sprintf("0.0.0.0:%d", cfg.APIPort),
				"default_mode":        "rule",
			},
		},
	}
}

// Deploy 上传配置、校验并重载 sing-box。
func (c *Client) Deploy(ctx context.Context, content []byte) error {
	if err := c.ssh.UploadFile(ctx, c.cfg.ConfigPath, string(content)); err != nil {
		return fmt.Errorf("上传配置失败: %w", err)
	}
	if out, err := c.ssh.Run(ctx, fmt.Sprintf("sing-box check -c %s 2>&1", shellQuote(c.cfg.ConfigPath))); err != nil {
		return fmt.Errorf("配置校验失败: %w (%s)", err, strings.TrimSpace(out))
	}
	if out, err := c.ssh.Run(ctx, "systemctl reload sing-box 2>&1; pkill -f 'sing-box run' 2>/dev/null; true"); err != nil {
		return fmt.Errorf("重载 sing-box 失败: %w (%s)", err, strings.TrimSpace(out))
	}
	return nil
}

// Restart 重启 sing-box 服务。
func (c *Client) Restart(ctx context.Context) error {
	_, err := c.ssh.Run(ctx, "systemctl restart sing-box 2>&1 || pkill -f 'sing-box run' 2>/dev/null || true")
	return err
}

// shellQuote quotes a string for shell use.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// GenerateClashProxy returns a Clash-compatible proxy JSON string for the sing-box server's mixed inbound.
// This is used to sync the server as a node into the nodes table, making it available for subscription output.
func (c *Client) GenerateClashProxy() (string, error) {
	proxy := map[string]any{
		"name":         "singbox-" + c.cfg.Host,
		"type":         "mixed",
		"server":       c.cfg.Host,
		"port":         c.cfg.SingboxPort,
		"udp":          true,
	}
	if c.cfg.SingboxUser != "" {
		proxy["username"] = c.cfg.SingboxUser
		proxy["password"] = c.cfg.SingboxUser
	}
	b, err := json.Marshal(proxy)
	if err != nil {
		return "", fmt.Errorf("marshal clash proxy: %w", err)
	}
	return string(b), nil
}