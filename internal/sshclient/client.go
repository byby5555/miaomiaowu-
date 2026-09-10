// Package sshclient provides a thin wrapper around golang.org/x/crypto/ssh for
// executing remote commands and transferring files over sane timeouts.
package sshclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Default timeouts.
const (
	DefaultConnectTimeout = 10 * time.Second
	DefaultCommandTimeout = 30 * time.Second
)

// Client is a single SSH session wrapper.
type Client struct {
	host        string
	port        int
	user        string
	authMethods []ssh.AuthMethod
	hostTimeout time.Duration
	cmdTimeout  time.Duration
}

// Options configures a Client.
type Options struct {
	Host           string
	Port           int
	User           string
	Password       string
	PrivateKey     string
	ConnectTimeout time.Duration
	CommandTimeout time.Duration
}

// New creates an SSH client from options.
func New(opts Options) (*Client, error) {
	if opts.Host == "" {
		return nil, errors.New("ssh host is required")
	}
	if opts.Port <= 0 {
		opts.Port = 22
	}
	if opts.User == "" {
		opts.User = "root"
	}
	if opts.ConnectTimeout <= 0 {
		opts.ConnectTimeout = DefaultConnectTimeout
	}
	if opts.CommandTimeout <= 0 {
		opts.CommandTimeout = DefaultCommandTimeout
	}

	var auths []ssh.AuthMethod
	switch {
	case opts.PrivateKey != "":
		signer, err := ssh.ParsePrivateKey([]byte(opts.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	case opts.Password != "":
		auths = append(auths, ssh.Password(opts.Password))
	default:
		return nil, errors.New("ssh password or private key is required")
	}

	return &Client{
		host:        opts.Host,
		port:        opts.Port,
		user:        opts.User,
		authMethods: auths,
		hostTimeout: opts.ConnectTimeout,
		cmdTimeout:  opts.CommandTimeout,
	}, nil
}

// dial establishes an SSH connection.
func (c *Client) dial(ctx context.Context) (*ssh.Client, error) {
	addr := net.JoinHostPort(c.host, fmt.Sprintf("%d", c.port))
	conn, err := net.DialTimeout("tcp", addr, c.hostTimeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User:            c.user,
		Auth:            c.authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // admin-managed remote inventory
		Timeout:         c.hostTimeout,
	})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh handshake: %w", err)
	}
	return ssh.NewClient(sshConn, chans, reqs), nil
}

// Run executes a remote command and returns combined stdout+stderr.
func (c *Client) Run(ctx context.Context, command string) (string, error) {
	if c == nil {
		return "", errors.New("ssh client is nil")
	}
	sshClient, err := c.dial(ctx)
	if err != nil {
		return "", err
	}
	defer sshClient.Close()

	session, err := sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("open session: %w", err)
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf
	session.Stderr = &buf

	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()
	select {
	case <-ctx.Done():
		_ = session.Close()
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			return buf.String(), fmt.Errorf("remote command failed: %w (output: %s)", err, strings.TrimSpace(buf.String()))
		}
		return buf.String(), nil
	}
}

// UploadFile writes content to a remote file atomically (temp file then rename).
func (c *Client) UploadFile(ctx context.Context, remotePath, content string) error {
	if c == nil {
		return errors.New("ssh client is nil")
	}
	if remotePath == "" {
		return errors.New("remote path is required")
	}
	tmp := remotePath + ".tmp"
	cmd := fmt.Sprintf("cat > %s << 'SINGBOX_EOF'\n%s\nSINGBOX_EOF\nmv %s %s", shellQuote(tmp), content, shellQuote(tmp), shellQuote(remotePath))
	_, err := c.Run(ctx, cmd)
	return err
}

// DownloadFile reads a remote file.
func (c *Client) DownloadFile(ctx context.Context, remotePath string) (string, error) {
	if c == nil {
		return "", errors.New("ssh client is nil")
	}
	if remotePath == "" {
		return "", errors.New("remote path is required")
	}
	return c.Run(ctx, fmt.Sprintf("cat %s", shellQuote(remotePath)))
}

// shellQuote quotes a string for shell use.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}