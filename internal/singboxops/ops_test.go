package singboxops

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildConfig(t *testing.T) {
	cfg := Remote{
		Host:        "1.2.3.4",
		SSHUser:     "root",
		ConfigPath:  "/etc/sing-box/config.json",
		APIPort:     9090,
		SingboxPort: 7890,
		SingboxUser: "alice",
	}
	data, err := json.Marshal(buildConfig(cfg))
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	s := string(data)
	// 必须包含入站/出站/API 配置
	for _, want := range []string{`"listen_port":7890`, `"external_controller":"0.0.0.0:9090"`, `"type":"direct"`, `"username":"alice"`} {
		if !strings.Contains(s, want) {
			t.Errorf("config missing %q, got: %s", want, s)
		}
	}
}

func TestClientRequiresHost(t *testing.T) {
	if _, err := New(Remote{}); err == nil {
		t.Fatal("expected error for empty host")
	}
}

// TestStatusJSONShape ensures the Status type marshals with the expected field names.
func TestStatusJSONShape(t *testing.T) {
	st := Status{SSHOK: true, Running: true, Version: "1.11.0", ConfigExists: true, Message: "OK"}
	b, _ := json.Marshal(st)
	s := string(b)
	for _, want := range []string{`"ssh_ok":true`, `"running":true`, `"version":"1.11.0"`, `"config_exists":true`} {
		if !strings.Contains(s, want) {
			t.Errorf("status JSON missing %q: %s", want, s)
		}
	}
	_ = context.Background()
}