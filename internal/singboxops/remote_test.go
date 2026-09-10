package singboxops

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestSSHRemoteConnection tests against a real SSH server.
// Set TEST_SSH_HOST, TEST_SSH_PORT, TEST_SSH_USER, TEST_SSH_PASS to enable.
func TestSSHRemoteConnection(t *testing.T) {
	host := os.Getenv("TEST_SSH_HOST")
	if host == "" {
		t.Skip("TEST_SSH_HOST not set, skipping remote SSH test")
	}
	port := 22
	if p := os.Getenv("TEST_SSH_PORT"); p != "" {
		port = 0
		for _, c := range p {
			port = port*10 + int(c-'0')
		}
	}
	user := os.Getenv("TEST_SSH_USER")
	if user == "" {
		user = "root"
	}
	pass := os.Getenv("TEST_SSH_PASS")

	client, err := New(Remote{
		Host:        host,
		Port:        port,
		SSHUser:     user,
		AuthType:    "password",
		AuthData:    pass,
		ConfigPath:  "/etc/sing-box/config.json",
		APIPort:     9090,
		SingboxPort: 7890,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Test connection
	if err := client.TestConnection(ctx); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	t.Log("SSH connection OK")

	// Get status
	st, _ := client.GetStatus(ctx)
	t.Logf("Status: SSHOK=%v Running=%v Version=%q ConfigExists=%v Message=%q",
		st.SSHOK, st.Running, st.Version, st.ConfigExists, st.Message)

	// Generate config
	cfg, err := client.GenerateConfig()
	if err != nil {
		t.Fatalf("GenerateConfig: %v", err)
	}
	t.Logf("Config generated: %d bytes", len(cfg))
}