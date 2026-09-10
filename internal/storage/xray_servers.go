package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// XrayServer represents a local or remote Xray/sing-box instance.
// We keep the table name xray_servers for mmwX compatibility but use it for sing-box too.
type XrayServer struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Host              string    `json:"host"`
	Port              int       `json:"port"`
	Description       string    `json:"description"`
	IsLocal           bool      `json:"is_local"`
	IsPrimary         bool      `json:"is_primary"`
	ProcessID         int       `json:"process_id"`
	ConfigPath        string    `json:"config_path"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	TrafficLimit      int64     `json:"traffic_limit"`
	TrafficResetDay   int       `json:"traffic_reset_day"`
	TrafficUsedOffset int64     `json:"traffic_used_offset"`
}

const xrayServersSchema = `
CREATE TABLE IF NOT EXISTS xray_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    host TEXT NOT NULL,
    port INTEGER NOT NULL,
    description TEXT,
    is_local INTEGER NOT NULL DEFAULT 0,
    is_primary INTEGER NOT NULL DEFAULT 0,
    process_id INTEGER NOT NULL DEFAULT 0,
    config_path TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    traffic_limit INTEGER NOT NULL DEFAULT 0,
    traffic_reset_day INTEGER NOT NULL DEFAULT 0,
    traffic_used_offset INTEGER NOT NULL DEFAULT 0,
    UNIQUE(host, port)
);
`

func (r *TrafficRepository) migrateXrayServers() error {
	_, err := r.db.Exec(xrayServersSchema)
	return err
}

var ErrXrayServerNotFound = errors.New("xray server not found")

func (r *TrafficRepository) CreateXrayServer(ctx context.Context, s *XrayServer) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO xray_servers (name, host, port, description, is_local, is_primary, config_path, traffic_limit, traffic_reset_day, traffic_used_offset) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.Name, s.Host, s.Port, s.Description, boolToIntXray(s.IsLocal), boolToIntXray(s.IsPrimary), s.ConfigPath, s.TrafficLimit, s.TrafficResetDay, s.TrafficUsedOffset)
	if err != nil {
		return 0, fmt.Errorf("create xray server: %w", err)
	}
	return res.LastInsertId()
}

func (r *TrafficRepository) ListXrayServers(ctx context.Context) ([]XrayServer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, host, port, COALESCE(description,''), is_local, is_primary, process_id, COALESCE(config_path,''), created_at, updated_at, traffic_limit, traffic_reset_day, traffic_used_offset FROM xray_servers ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list xray servers: %w", err)
	}
	defer rows.Close()
	var out []XrayServer
	for rows.Next() {
		var s XrayServer
		var isLocal, isPrimary int
		if err := rows.Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Description, &isLocal, &isPrimary, &s.ProcessID, &s.ConfigPath, &s.CreatedAt, &s.UpdatedAt, &s.TrafficLimit, &s.TrafficResetDay, &s.TrafficUsedOffset); err != nil {
			return nil, err
		}
		s.IsLocal = isLocal != 0
		s.IsPrimary = isPrimary != 0
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) GetXrayServer(ctx context.Context, id int64) (XrayServer, error) {
	var s XrayServer
	var isLocal, isPrimary int
	err := r.db.QueryRowContext(ctx, `SELECT id, name, host, port, COALESCE(description,''), is_local, is_primary, process_id, COALESCE(config_path,''), created_at, updated_at, traffic_limit, traffic_reset_day, traffic_used_offset FROM xray_servers WHERE id = ?`, id).
		Scan(&s.ID, &s.Name, &s.Host, &s.Port, &s.Description, &isLocal, &isPrimary, &s.ProcessID, &s.ConfigPath, &s.CreatedAt, &s.UpdatedAt, &s.TrafficLimit, &s.TrafficResetDay, &s.TrafficUsedOffset)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrXrayServerNotFound
	}
	s.IsLocal = isLocal != 0
	s.IsPrimary = isPrimary != 0
	return s, err
}

func (r *TrafficRepository) UpdateXrayServer(ctx context.Context, s *XrayServer) error {
	_, err := r.db.ExecContext(ctx, `UPDATE xray_servers SET name=?, host=?, port=?, description=?, is_local=?, is_primary=?, config_path=?, traffic_limit=?, traffic_reset_day=?, traffic_used_offset=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		s.Name, s.Host, s.Port, s.Description, boolToIntXray(s.IsLocal), boolToIntXray(s.IsPrimary), s.ConfigPath, s.TrafficLimit, s.TrafficResetDay, s.TrafficUsedOffset, s.ID)
	return err
}

func (r *TrafficRepository) DeleteXrayServer(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM xray_servers WHERE id = ?`, id)
	return err
}

// --- NodeTraffic (per-inbound/outbound traffic tracking) ---

type NodeTraffic struct {
	ID            int64     `json:"id"`
	ServerID      int64     `json:"server_id"`
	Tag           string    `json:"tag"`
	Type          string    `json:"type"` // inbound | outbound
	Uplink        int64     `json:"uplink"`
	Downlink      int64     `json:"downlink"`
	TotalUplink   int64     `json:"total_uplink"`
	TotalDownlink int64     `json:"total_downlink"`
	LastUplink    int64     `json:"last_uplink"`
	LastDownlink  int64     `json:"last_downlink"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const nodeTrafficSchema = `
CREATE TABLE IF NOT EXISTS node_traffic (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('inbound', 'outbound')),
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    total_uplink INTEGER NOT NULL DEFAULT 0,
    total_downlink INTEGER NOT NULL DEFAULT 0,
    last_uplink INTEGER NOT NULL DEFAULT 0,
    last_downlink INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, tag, type),
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateNodeTraffic() error {
	_, err := r.db.Exec(nodeTrafficSchema)
	return err
}

// --- UserTraffic (per-user traffic on a server) ---

type UserTraffic struct {
	ID            int64     `json:"id"`
	ServerID      int64     `json:"server_id"`
	Username      string    `json:"username"`
	Uplink        int64     `json:"uplink"`
	Downlink      int64     `json:"downlink"`
	TotalUplink   int64     `json:"total_uplink"`
	TotalDownlink int64     `json:"total_downlink"`
	LastUplink    int64     `json:"last_uplink"`
	LastDownlink  int64     `json:"last_downlink"`
	CycleStart    time.Time `json:"cycle_start"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const userTrafficSchema = `
CREATE TABLE IF NOT EXISTS user_traffic (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    username TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    total_uplink INTEGER NOT NULL DEFAULT 0,
    total_downlink INTEGER NOT NULL DEFAULT 0,
    last_uplink INTEGER NOT NULL DEFAULT 0,
    last_downlink INTEGER NOT NULL DEFAULT 0,
    cycle_start TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, username),
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateUserTraffic() error {
	_, err := r.db.Exec(userTrafficSchema)
	return err
}

// --- UserEmailTraffic (per-email traffic on a server) ---

type UserEmailTraffic struct {
	ID            int64     `json:"id"`
	ServerID      int64     `json:"server_id"`
	Email         string    `json:"email"`
	Uplink        int64     `json:"uplink"`
	Downlink      int64     `json:"downlink"`
	TotalUplink   int64     `json:"total_uplink"`
	TotalDownlink int64     `json:"total_downlink"`
	LastUplink    int64     `json:"last_uplink"`
	LastDownlink  int64     `json:"last_downlink"`
	CycleStart    time.Time `json:"cycle_start"`
	UpdatedAt     time.Time `json:"updated_at"`
}

const userEmailTrafficSchema = `
CREATE TABLE IF NOT EXISTS user_email_traffic (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    email TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    total_uplink INTEGER NOT NULL DEFAULT 0,
    total_downlink INTEGER NOT NULL DEFAULT 0,
    last_uplink INTEGER NOT NULL DEFAULT 0,
    last_downlink INTEGER NOT NULL DEFAULT 0,
    cycle_start TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, email),
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateUserEmailTraffic() error {
	_, err := r.db.Exec(userEmailTrafficSchema)
	return err
}

// --- TrafficSnapshots (daily server traffic snapshots) ---

type TrafficSnapshot struct {
	ID                    int64     `json:"id"`
	ServerID              int64     `json:"server_id"`
	Date                  string    `json:"date"`
	InboundUplink         int64     `json:"inbound_uplink"`
	InboundDownlink       int64     `json:"inbound_downlink"`
	OutboundUplink        int64     `json:"outbound_uplink"`
	OutboundDownlink      int64     `json:"outbound_downlink"`
	UserUplink            int64     `json:"user_uplink"`
	UserDownlink          int64     `json:"user_downlink"`
	CreatedAt             time.Time `json:"created_at"`
}

const trafficSnapshotsSchema = `
CREATE TABLE IF NOT EXISTS traffic_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    date TEXT NOT NULL,
    inbound_uplink INTEGER NOT NULL DEFAULT 0,
    inbound_downlink INTEGER NOT NULL DEFAULT 0,
    outbound_uplink INTEGER NOT NULL DEFAULT 0,
    outbound_downlink INTEGER NOT NULL DEFAULT 0,
    user_uplink INTEGER NOT NULL DEFAULT 0,
    user_downlink INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, date),
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateTrafficSnapshots() error {
	_, err := r.db.Exec(trafficSnapshotsSchema)
	return err
}

// --- BatchInbounds / BatchOutbounds ---

type BatchInbound struct {
	ID        int64     `json:"id"`
	BatchID   string    `json:"batch_id"`
	Tag       string    `json:"tag"`
	ServerID  int64     `json:"server_id"`
	Protocol  string    `json:"protocol"`
	Port      int       `json:"port"`
	CreatedAt time.Time `json:"created_at"`
}

const batchInboundsSchema = `
CREATE TABLE IF NOT EXISTS batch_inbounds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    server_id INTEGER NOT NULL,
    protocol TEXT NOT NULL,
    port INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateBatchInbounds() error {
	_, err := r.db.Exec(batchInboundsSchema)
	return err
}

type BatchOutbound struct {
	ID        int64     `json:"id"`
	BatchID   string    `json:"batch_id"`
	Tag       string    `json:"tag"`
	ServerID  int64     `json:"server_id"`
	Protocol  string    `json:"protocol"`
	CreatedAt time.Time `json:"created_at"`
}

const batchOutboundsSchema = `
CREATE TABLE IF NOT EXISTS batch_outbounds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    batch_id TEXT NOT NULL,
    tag TEXT NOT NULL,
    server_id INTEGER NOT NULL,
    protocol TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES xray_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateBatchOutbounds() error {
	_, err := r.db.Exec(batchOutboundsSchema)
	return err
}

// --- UserInboundConfigs / UserOutbounds ---

type UserInboundConfig struct {
	ID              int64     `json:"id"`
	Username        string    `json:"username"`
	ServerID        int64     `json:"server_id"`
	InboundTag      string    `json:"inbound_tag"`
	Protocol        string    `json:"protocol"`
	CredentialJSON  string    `json:"credential_json"`
	CreatedAt       time.Time `json:"created_at"`
}

const userInboundConfigsSchema = `
CREATE TABLE IF NOT EXISTS user_inbound_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    server_id INTEGER NOT NULL,
    inbound_tag TEXT NOT NULL,
    protocol TEXT NOT NULL,
    credential_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateUserInboundConfigs() error {
	_, err := r.db.Exec(userInboundConfigsSchema)
	return err
}

type UserOutbound struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	ServerID     int64     `json:"server_id"`
	InboundTag   string    `json:"inbound_tag"`
	OutboundTag  string    `json:"outbound_tag"`
	OutboundJSON string    `json:"outbound_json"`
	CreatedAt    time.Time `json:"created_at"`
}

const userOutboundsSchema = `
CREATE TABLE IF NOT EXISTS user_outbounds (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    server_id INTEGER NOT NULL,
    inbound_tag TEXT NOT NULL,
    outbound_tag TEXT NOT NULL,
    outbound_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateUserOutbounds() error {
	_, err := r.db.Exec(userOutboundsSchema)
	return err
}

func boolToIntXray(b bool) int {
	if b {
		return 1
	}
	return 0
}

