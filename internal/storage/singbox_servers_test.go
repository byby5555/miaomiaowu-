package storage

import (
	"context"
	"errors"
	"testing"
)

func TestSingboxServerCRUD(t *testing.T) {
	ctx := context.Background()
	repo := newTestRepo(t)
	defer repo.db.Close()

	// CREATE
	srv, err := repo.CreateSingboxServer(ctx, SingboxServer{
		Name:        "test-server",
		Host:        "127.0.0.1",
		Port:        2222,
		SSHUser:     "root",
		ConfigPath:  "/etc/sing-box/config.json",
		APIPort:     9090,
		SingboxPort: 7890,
		Enabled:     true,
	}, SingboxServerAuth{AuthType: "password", AuthData: "secret"})
	if err != nil {
		t.Fatalf("CreateSingboxServer: %v", err)
	}
	if srv.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if !srv.Enabled {
		t.Fatal("expected enabled")
	}
	// auth data must NOT be returned
	if srv.AuthData != "" {
		t.Fatalf("auth data leaked to non-sensitive struct: %q", srv.AuthData)
	}

	// duplicate name
	if _, err := repo.CreateSingboxServer(ctx, SingboxServer{Name: "test-server", Host: "1.2.3.4"}, SingboxServerAuth{AuthType: "password", AuthData: "x"}); !errors.Is(err, ErrSingboxServerExists) {
		t.Fatalf("expected ErrSingboxServerExists, got %v", err)
	}

	// GET
	got, err := repo.GetSingboxServer(ctx, srv.ID)
	if err != nil {
		t.Fatalf("GetSingboxServer: %v", err)
	}
	if got.Name != "test-server" || got.Host != "127.0.0.1" {
		t.Fatalf("unexpected server: %+v", got)
	}

	// GET AUTH (sensitive fields via separate method)
	auth, err := repo.GetSingboxServerAuth(ctx, srv.ID)
	if err != nil {
		t.Fatalf("GetSingboxServerAuth: %v", err)
	}
	if auth.AuthData != "secret" || auth.AuthType != "password" {
		t.Fatalf("unexpected auth: %+v", auth)
	}

	// LIST
	list, err := repo.ListSingboxServers(ctx)
	if err != nil {
		t.Fatalf("ListSingboxServers: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 server, got %d", len(list))
	}

	// UPDATE (without auth -> auth unchanged)
	got.Port = 2223
	updated, err := repo.UpdateSingboxServer(ctx, got, nil)
	if err != nil {
		t.Fatalf("UpdateSingboxServer: %v", err)
	}
	if updated.Port != 2223 {
		t.Fatalf("expected port 2223, got %d", updated.Port)
	}
	auth2, _ := repo.GetSingboxServerAuth(ctx, srv.ID)
	if auth2.AuthData != "secret" {
		t.Fatalf("auth data should remain unchanged, got %q", auth2.AuthData)
	}

	// UPDATE with new auth
	got2 := updated
	got2.Host = "10.0.0.1"
	updated2, err := repo.UpdateSingboxServer(ctx, got2, &SingboxServerAuth{AuthType: "private_key", AuthData: "keydata"})
	if err != nil {
		t.Fatalf("UpdateSingboxServer with auth: %v", err)
	}
	if updated2.Host != "10.0.0.1" {
		t.Fatalf("expected host update, got %+v", updated2)
	}
	auth3, _ := repo.GetSingboxServerAuth(ctx, srv.ID)
	if auth3.AuthType != "private_key" || auth3.AuthData != "keydata" {
		t.Fatalf("auth not updated: %+v", auth3)
	}

	// DELETE
	if err := repo.DeleteSingboxServer(ctx, srv.ID); err != nil {
		t.Fatalf("DeleteSingboxServer: %v", err)
	}
	if _, err := repo.GetSingboxServer(ctx, srv.ID); !errors.Is(err, ErrSingboxServerNotFound) {
		t.Fatalf("expected ErrSingboxServerNotFound after delete, got %v", err)
	}

	// SetSingboxServerStatus
	srv2, _ := repo.CreateSingboxServer(ctx, SingboxServer{Name: "s2", Host: "h2"}, SingboxServerAuth{AuthType: "password", AuthData: "x"})
	if err := repo.SetSingboxServerStatus(ctx, srv2.ID, "online"); err != nil {
		t.Fatalf("SetSingboxServerStatus: %v", err)
	}
	got3, _ := repo.GetSingboxServer(ctx, srv2.ID)
	if got3.HasStatus != "online" {
		t.Fatalf("expected status online, got %q", got3.HasStatus)
	}
}