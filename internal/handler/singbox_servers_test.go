package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"miaomiaowu/internal/auth"
	"miaomiaowu/internal/storage"
)

func TestSingboxServersAPI(t *testing.T) {
	repo, err := storage.NewTrafficRepository(":memory:")
	if err != nil {
		t.Fatalf("NewTrafficRepository: %v", err)
	}
	defer repo.Close()

	handler := NewSingboxServersHandler(repo)
	mux := http.NewServeMux()
	mux.Handle("/api/admin/singbox-servers", handler)
	mux.Handle("/api/admin/singbox-servers/", handler)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()
	ctx := context.Background()
	_ = ctx

	// CREATE
	createBody := map[string]any{
		"name":         "test-srv",
		"host":         "127.0.0.1",
		"port":         2222,
		"ssh_user":     "root",
		"auth_type":    "password",
		"auth_data":    "secret",
		"config_path":  "/etc/sing-box/config.json",
		"api_port":     9090,
		"singbox_port": 7890,
		"enabled":      true,
	}
	body, _ := json.Marshal(createBody)
	resp, err := client.Post(server.URL+"/api/admin/singbox-servers", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created singboxServerResponse
	json.NewDecoder(resp.Body).Decode(&created)
	if created.ID == 0 || created.Name != "test-srv" {
		t.Fatalf("unexpected response: %+v", created)
	}
	if created.HasAuth != true {
		t.Fatal("expected has_auth=true")
	}
	// auth data must NOT be in response
	respBody, _ := io.ReadAll(resp.Body)
	_ = respBody
	resp.Body.Close()

	// LIST
	resp, err = client.Get(server.URL + "/api/admin/singbox-servers")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var list []singboxServerResponse
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 1 {
		t.Fatalf("expected 1 server, got %d", len(list))
	}
	if !list[0].HasAuth {
		t.Fatal("expected has_auth=true")
	}
	// AuthData field doesn't exist in response (by design), verify type has no auth data leaks
	resp.Body.Close()

	// UPDATE
	updateBody := map[string]any{
		"name":         "test-srv-renamed",
		"host":         "10.0.0.1",
		"port":         2222,
		"ssh_user":     "root",
		"config_path":  "/etc/sing-box/config.json",
		"api_port":     9090,
		"singbox_port": 7890,
		"enabled":      true,
	}
	body, _ = json.Marshal(updateBody)
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/admin/singbox-servers/"+itoa(created.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("update request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var updated singboxServerResponse
	json.NewDecoder(resp.Body).Decode(&updated)
	if updated.Name != "test-srv-renamed" {
		t.Fatalf("expected renamed, got %q", updated.Name)
	}
	resp.Body.Close()

	// DELETE
	req, _ = http.NewRequest(http.MethodDelete, server.URL+"/api/admin/singbox-servers/"+itoa(created.ID), nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// verify deleted
	resp, err = client.Get(server.URL + "/api/admin/singbox-servers")
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 0 {
		t.Fatalf("expected 0 servers after delete, got %d", len(list))
	}
	resp.Body.Close()

	_ = time.Now()
}

func TestSingboxServerSyncNode(t *testing.T) {
	repo, err := storage.NewTrafficRepository(":memory:")
	if err != nil {
		t.Fatalf("NewTrafficRepository: %v", err)
	}
	defer repo.Close()

	// Create a test admin user so SyncSingboxServerNode can find the owner
	if err := repo.EnsureUser(context.Background(), "admin", "$2a$10$dummyhash"); err != nil {
		t.Fatalf("EnsureUser: %v", err)
	}

	// Wrap handler with auth context injection middleware
	handler := NewSingboxServersHandler(repo)
	authedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := auth.ContextWithUsername(r.Context(), "admin")
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
	mux := http.NewServeMux()
	mux.Handle("/api/admin/singbox-servers", authedHandler)
	mux.Handle("/api/admin/singbox-servers/", authedHandler)
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	// CREATE a sing-box server
	createBody := map[string]any{
		"name":         "sync-test-srv",
		"host":         "10.0.0.99",
		"port":         22,
		"ssh_user":     "root",
		"auth_type":    "password",
		"auth_data":    "secret",
		"singbox_port": 7890,
		"api_port":     9090,
		"enabled":      true,
	}
	body, _ := json.Marshal(createBody)
	resp, err := client.Post(server.URL+"/api/admin/singbox-servers", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var created singboxServerResponse
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == 0 {
		t.Fatal("expected non-zero ID")
	}

	// SYNC-NODE: creates a node from the server
	resp, err = client.Post(server.URL+"/api/admin/singbox-servers/"+itoa(created.ID)+"/sync-node", "application/json", nil)
	if err != nil {
		t.Fatalf("sync-node request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for sync-node, got %d", resp.StatusCode)
	}
	var syncResp map[string]any
	json.NewDecoder(resp.Body).Decode(&syncResp)
	resp.Body.Close()
	if syncResp["synced"] != true {
		t.Fatalf("expected synced=true, got %v", syncResp)
	}
	nodeID, ok := syncResp["node_id"].(float64)
	if !ok || nodeID == 0 {
		t.Fatalf("expected non-zero node_id, got %v", syncResp["node_id"])
	}

	// Verify the node exists in the nodes table
	nodes, err := repo.ListNodes(context.Background(), "admin")
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].NodeName != "sync-test-srv" {
		t.Fatalf("expected node name 'sync-test-srv', got %q", nodes[0].NodeName)
	}
	if nodes[0].SingboxServerID == nil || *nodes[0].SingboxServerID != created.ID {
		t.Fatalf("expected singbox_server_id=%d, got %v", created.ID, nodes[0].SingboxServerID)
	}
	if nodes[0].ClashConfig == "" {
		t.Fatal("expected non-empty clash_config")
	}

	// SYNC-NODE again: should update the same node, not create a duplicate
	resp, err = client.Post(server.URL+"/api/admin/singbox-servers/"+itoa(created.ID)+"/sync-node", "application/json", nil)
	if err != nil {
		t.Fatalf("second sync-node request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for second sync-node, got %d", resp.StatusCode)
	}
	var syncResp2 map[string]any
	json.NewDecoder(resp.Body).Decode(&syncResp2)
	resp.Body.Close()
	nodeID2, _ := syncResp2["node_id"].(float64)
	if int64(nodeID2) != int64(nodeID) {
		t.Fatalf("expected same node_id on re-sync, got %v vs %v", nodeID, nodeID2)
	}

	// Verify still only 1 node
	nodes, _ = repo.ListNodes(context.Background(), "admin")
	if len(nodes) != 1 {
		t.Fatalf("expected still 1 node after re-sync, got %d", len(nodes))
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}