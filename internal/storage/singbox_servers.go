package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SingboxServer represents a remote sing-box server managed via SSH.
type SingboxServer struct {
	ID            int64
	Name          string
	Host          string
	Port          int
	SSHUser       string
	AuthType      string // "password" | "private_key"
	AuthData      string // password or private key content (never returned to frontend)
	ConfigPath    string // remote config file path, default /etc/sing-box/config.json
	APIPort       int    // sing-box control API port, default 9090
	SingboxPort   int    // inbound port, default 7890
	SingboxUser   string // user for connecting through sing-box
	Enabled       bool
	HasStatus     string // last known status message (online/offline/error)
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SingboxServerAuth is the sensitive auth payload. Never JSON-marshaled to frontend directly.
type SingboxServerAuth struct {
	AuthType string
	AuthData string
}

const singboxServersSchema = `
CREATE TABLE IF NOT EXISTS singbox_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    host TEXT NOT NULL,
    port INTEGER NOT NULL DEFAULT 22,
    ssh_user TEXT NOT NULL DEFAULT 'root',
    auth_type TEXT NOT NULL DEFAULT 'password',
    auth_data TEXT NOT NULL DEFAULT '',
    config_path TEXT NOT NULL DEFAULT '/etc/sing-box/config.json',
    api_port INTEGER NOT NULL DEFAULT 9090,
    singbox_port INTEGER NOT NULL DEFAULT 7890,
    singbox_user TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    has_status TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// migrateSingboxServers creates the singbox_servers table.
func (r *TrafficRepository) migrateSingboxServers() error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}
	if _, err := r.db.Exec(singboxServersSchema); err != nil {
		return fmt.Errorf("migrate singbox_servers: %w", err)
	}
	return nil
}

// listSingboxServersColumns lists the columns that can be returned to the frontend
// (excluding sensitive auth_data).
func listSingboxServersColumns() string {
	return `id, name, host, port, ssh_user, config_path, api_port, singbox_port, singbox_user, enabled, COALESCE(has_status,''), created_at, updated_at`
}

// scanSingboxServer scans a row into SingboxServer (non-sensitive fields).
func scanSingboxServer(row interface{ Scan(...any) error }) (SingboxServer, error) {
	var s SingboxServer
	var enabled int
	if err := row.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.SSHUser, &s.ConfigPath, &s.APIPort, &s.SingboxPort, &s.SingboxUser, &enabled, &s.HasStatus, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return s, err
	}
	s.Enabled = enabled != 0
	return s, nil
}

// ListSingboxServers returns all singbox servers (without auth data).
func (r *TrafficRepository) ListSingboxServers(ctx context.Context) ([]SingboxServer, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("traffic repository not initialized")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+listSingboxServersColumns()+` FROM singbox_servers ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list singbox servers: %w", err)
	}
	defer rows.Close()

	var servers []SingboxServer
	for rows.Next() {
		s, err := scanSingboxServer(rows)
		if err != nil {
			return nil, fmt.Errorf("scan singbox server: %w", err)
		}
		servers = append(servers, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate singbox servers: %w", err)
	}
	return servers, nil
}

// GetSingboxServer returns a single server (without auths).
func (r *TrafficRepository) GetSingboxServer(ctx context.Context, id int64) (SingboxServer, error) {
	var s SingboxServer
	if r == nil || r.db == nil {
		return s, errors.New("traffic repository not initialized")
	}
	if id <= 0 {
		return s, errors.New("server id is required")
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+listSingboxServersColumns()+` FROM singbox_servers WHERE id = ? LIMIT 1`, id)
	s, err := scanSingboxServer(row)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrSingboxServerNotFound
	}
	if err != nil {
		return s, fmt.Errorf("get singbox server: %w", err)
	}
	return s, nil
}

// GetSingboxServerAuth returns the sensitive auth fields for a server (used by handler for SSH ops).
func (r *TrafficRepository) GetSingboxServerAuth(ctx context.Context, id int64) (SingboxServerAuth, error) {
	var auth SingboxServerAuth
	if r == nil || r.db == nil {
		return auth, errors.New("traffic repository not initialized")
	}
	if id <= 0 {
		return auth, errors.New("server id is required")
	}
	err := r.db.QueryRowContext(ctx, `SELECT auth_type, auth_data FROM singbox_servers WHERE id = ? LIMIT 1`, id).Scan(&auth.AuthType, &auth.AuthData)
	if errors.Is(err, sql.ErrNoRows) {
		return auth, ErrSingboxServerNotFound
	}
	if err != nil {
		return auth, fmt.Errorf("get singbox server auth: %w", err)
	}
	return auth, nil
}

// CreateSingboxServer inserts a new sing-box server.
func (r *TrafficRepository) CreateSingboxServer(ctx context.Context, s SingboxServer, auth SingboxServerAuth) (SingboxServer, error) {
	if r == nil || r.db == nil {
		return SingboxServer{}, errors.New("traffic repository not initialized")
	}

	s.Name = strings.TrimSpace(s.Name)
	s.Host = strings.TrimSpace(s.Host)
	s.SSHUser = strings.TrimSpace(s.SSHUser)
	if s.SSHUser == "" {
		s.SSHUser = "root"
	}
	s.ConfigPath = strings.TrimSpace(s.ConfigPath)
	if s.ConfigPath == "" {
		s.ConfigPath = "/etc/sing-box/config.json"
	}
	if s.Port <= 0 {
		s.Port = 22
	}
	if s.APIPort <= 0 {
		s.APIPort = 9090
	}
	if s.SingboxPort <= 0 {
		s.SingboxPort = 7890
	}
	if auth.AuthType == "" {
		auth.AuthType = "password"
	}
	if auth.AuthType != "password" && auth.AuthType != "private_key" {
		return SingboxServer{}, errors.New("invalid auth type")
	}
	if auth.AuthData == "" {
		return SingboxServer{}, errors.New("auth data is required")
	}
	if s.Name == "" {
		return SingboxServer{}, errors.New("server name is required")
	}
	if s.Host == "" {
		return SingboxServer{}, errors.New("server host is required")
	}

	enabled := 0
	if s.Enabled {
		enabled = 1
	}

	res, err := r.db.ExecContext(ctx, `INSERT INTO singbox_servers (name, host, port, ssh_user, auth_type, auth_data, config_path, api_port, singbox_port, singbox_user, enabled, has_status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '')`, s.Name, s.Host, s.Port, s.SSHUser, auth.AuthType, auth.AuthData, s.ConfigPath, s.APIPort, s.SingboxPort, s.SingboxUser, enabled)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return SingboxServer{}, ErrSingboxServerExists
		}
		return SingboxServer{}, fmt.Errorf("create singbox server: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return SingboxServer{}, fmt.Errorf("fetch singbox server id: %w", err)
	}
	return r.GetSingboxServer(ctx, id)
}

// UpdateSingboxServer updates an existing server. If auth is nil, auth fields are left unchanged.
func (r *TrafficRepository) UpdateSingboxServer(ctx context.Context, s SingboxServer, auth *SingboxServerAuth) (SingboxServer, error) {
	if r == nil || r.db == nil {
		return SingboxServer{}, errors.New("traffic repository not initialized")
	}
	if s.ID <= 0 {
		return SingboxServer{}, errors.New("server id is required")
	}
	s.Name = strings.TrimSpace(s.Name)
	s.Host = strings.TrimSpace(s.Host)
	s.SSHUser = strings.TrimSpace(s.SSHUser)
	if s.SSHUser == "" {
		s.SSHUser = "root"
	}
	if s.Port <= 0 {
		s.Port = 22
	}
	if s.APIPort <= 0 {
		s.APIPort = 9090
	}
	if s.SingboxPort <= 0 {
		s.SingboxPort = 7890
	}
	if s.Name == "" {
		return SingboxServer{}, errors.New("server name is required")
	}
	if s.Host == "" {
		return SingboxServer{}, errors.New("server host is required")
	}
	enabled := 0
	if s.Enabled {
		enabled = 1
	}

	var res sql.Result
	var err error
	if auth != nil && auth.AuthData != "" {
		if auth.AuthType == "" {
			auth.AuthType = "password"
		}
		res, err = r.db.ExecContext(ctx, `UPDATE singbox_servers SET name=?, host=?, port=?, ssh_user=?, auth_type=?, auth_data=?, config_path=?, api_port=?, singbox_port=?, singbox_user=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, s.Name, s.Host, s.Port, s.SSHUser, auth.AuthType, auth.AuthData, s.ConfigPath, s.APIPort, s.SingboxPort, s.SingboxUser, enabled, s.ID)
	} else {
		res, err = r.db.ExecContext(ctx, `UPDATE singbox_servers SET name=?, host=?, port=?, ssh_user=?, config_path=?, api_port=?, singbox_port=?, singbox_user=?, enabled=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, s.Name, s.Host, s.Port, s.SSHUser, s.ConfigPath, s.APIPort, s.SingboxPort, s.SingboxUser, enabled, s.ID)
	}
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return SingboxServer{}, ErrSingboxServerExists
		}
		return SingboxServer{}, fmt.Errorf("update singbox server: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return SingboxServer{}, fmt.Errorf("singbox server update rows affected: %w", err)
	}
	if affected == 0 {
		return SingboxServer{}, ErrSingboxServerNotFound
	}
	return r.GetSingboxServer(ctx, s.ID)
}

// DeleteSingboxServer removes a sing-box server.
func (r *TrafficRepository) DeleteSingboxServer(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}
	if id <= 0 {
		return errors.New("server id is required")
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM singbox_servers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete singbox server: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("singbox server delete rows affected: %w", err)
	}
	if affected == 0 {
		return ErrSingboxServerNotFound
	}
	return nil
}

// SetSingboxServerStatus updates the last known status string of a server.
func (r *TrafficRepository) SetSingboxServerStatus(ctx context.Context, id int64, status string) error {
	if r == nil || r.db == nil {
		return errors.New("traffic repository not initialized")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE singbox_servers SET has_status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

// SyncSingboxServerNode creates or updates a node in the nodes table for the given sing-box server.
// The node represents the server's mixed inbound as a Clash proxy.
// username specifies which user owns the synced node (should be the admin's username).
// clashConfig is the pre-built Clash proxy JSON string to store as the node's clash_config.
func (r *TrafficRepository) SyncSingboxServerNode(ctx context.Context, serverID int64, username, clashConfig string) (Node, error) {
	if r == nil || r.db == nil {
		return Node{}, errors.New("traffic repository not initialized")
	}
	if serverID <= 0 {
		return Node{}, errors.New("server id is required")
	}
	if username == "" {
		return Node{}, errors.New("username is required")
	}
	if clashConfig == "" {
		return Node{}, errors.New("clash config is required")
	}

	// Get the server to build node name / protocol
	srv, err := r.GetSingboxServer(ctx, serverID)
	if err != nil {
		return Node{}, err
	}

	nodeName := srv.Name
	protocol := "mixed"
	parsedConfig := fmt.Sprintf(`{"server":"%s","server_port":%d}`, srv.Host, srv.SingboxPort)

	// Check if a node already exists for this singbox_server_id
	var existingID int64
	err = r.db.QueryRowContext(ctx, `SELECT id FROM nodes WHERE singbox_server_id = ? AND username = ? LIMIT 1`, serverID, username).Scan(&existingID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Node{}, fmt.Errorf("query existing singbox node: %w", err)
	}

	if existingID > 0 {
		// Update existing node
		_, err := r.db.ExecContext(ctx, `UPDATE nodes SET node_name = ?, protocol = ?, parsed_config = ?, clash_config = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND username = ?`, nodeName, protocol, parsedConfig, clashConfig, existingID, username)
		if err != nil {
			return Node{}, fmt.Errorf("update singbox node: %w", err)
		}
		return r.GetNode(ctx, existingID, username)
	}

	// Create new node
	res, err := r.db.ExecContext(ctx, `INSERT INTO nodes (username, raw_url, node_name, protocol, parsed_config, clash_config, enabled, tag, tags, singbox_server_id) VALUES (?, '', ?, ?, ?, ?, 1, 'sing-box', '["sing-box"]', ?)`, username, nodeName, protocol, parsedConfig, clashConfig, serverID)
	if err != nil {
		return Node{}, fmt.Errorf("create singbox node: %w", err)
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return Node{}, fmt.Errorf("fetch singbox node id: %w", err)
	}
	return r.GetNode(ctx, newID, username)
}

// GetNodesBySingboxServerID returns all nodes associated with a sing-box server ID.
func (r *TrafficRepository) GetNodesBySingboxServerID(ctx context.Context, serverID int64) ([]Node, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("traffic repository not initialized")
	}
	if serverID <= 0 {
		return nil, errors.New("server id is required")
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, username, raw_url, node_name, protocol, parsed_config, clash_config, enabled, COALESCE(tag, 'personal'), COALESCE(original_server, ''), COALESCE(probe_server, ''), COALESCE(tags, '[]'), chain_proxy_node_id, COALESCE(relay_group_name,''), COALESCE(relay_group_node_ids,'[]'), COALESCE(probe_enabled, 0), singbox_server_id, created_at, updated_at FROM nodes WHERE singbox_server_id = ?`, serverID)
	if err != nil {
		return nil, fmt.Errorf("get nodes by singbox server id: %w", err)
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var node Node
		var enabled int
		var probeEnabled int
		var tagsJSON, relayGroupNodeIDsJSON string
		if err := rows.Scan(&node.ID, &node.Username, &node.RawURL, &node.NodeName, &node.Protocol, &node.ParsedConfig, &node.ClashConfig, &enabled, &node.Tag, &node.OriginalServer, &node.ProbeServer, &tagsJSON, &node.ChainProxyNodeID, &node.RelayGroupName, &relayGroupNodeIDsJSON, &probeEnabled, &node.SingboxServerID, &node.CreatedAt, &node.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan singbox node: %w", err)
		}
		node.Enabled = enabled != 0
		node.ProbeEnabled = probeEnabled != 0
		scanNodeTags(&node, tagsJSON)
		scanRelayGroupNodeIDs(&node, relayGroupNodeIDsJSON)
		nodes = append(nodes, node)
	}
	return nodes, rows.Err()
}