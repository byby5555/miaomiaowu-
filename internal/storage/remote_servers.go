package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RemoteServer represents a remote proxy server managed via agent token (push/pull model).
// Mirrors mmwX remote_servers table structure.
type RemoteServer struct {
	ID                    int64      `json:"id"`
	Name                  string     `json:"name"`
	Token                 string     `json:"-"` // never expose to frontend directly
	Status                string     `json:"status"` // pending | connected | offline
	LastHeartbeat         *time.Time `json:"last_heartbeat"`
	IPAddress             string     `json:"ip_address"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	BootTime              *time.Time `json:"boot_time"`
	XrayBootTime          *time.Time `json:"xray_boot_time"`
	BootCount             int        `json:"boot_count"`
	XrayBootCount         int        `json:"xray_boot_count"`
	TokenExpiresAt        *time.Time `json:"token_expires_at"`
	LastTokenRefresh      *time.Time `json:"last_token_refresh"`
	ConnectionMode        string     `json:"connection_mode"` // push | pull
	PullAddress           string     `json:"pull_address"`
	PullPort              int        `json:"pull_port"`
	PullToken             string     `json:"-"`
	LastPullAt            *time.Time `json:"last_pull_at"`
	PushFailCount         int        `json:"push_fail_count"`
	LastPushFail          *time.Time `json:"last_push_fail"`
	FallbackToPull        bool       `json:"fallback_to_pull"`
	FallbackAt            *time.Time `json:"fallback_at"`
	CurrentUploadSpeed    int        `json:"current_upload_speed"`
	CurrentDownloadSpeed  int        `json:"current_download_speed"`
	SpeedUpdatedAt        *time.Time `json:"speed_updated_at"`
	XrayRunning           bool       `json:"xray_running"`
	XrayVersion           string     `json:"xray_version"`
	XrayScannedAt         *time.Time `json:"xray_scanned_at"`
	ListenPort            int        `json:"listen_port"`
	TrafficLimit          int64      `json:"traffic_limit"`
	TrafficResetDay       int        `json:"traffic_reset_day"`
	Domain                string     `json:"domain"`
	IPAddressV6           string     `json:"ip_address_v6"`
	WarpInstalled         bool       `json:"warp_installed"`
	AgentToken            string     `json:"-"`
	AgentTokenExpiresAt   *time.Time `json:"agent_token_expires_at"`
	LastAgentTokenRefresh *time.Time `json:"last_agent_token_refresh"`
	Use443                bool       `json:"use_443"`
	StealMode             string     `json:"steal_mode"`
	SiteType              string     `json:"site_type"`
	SiteValue             string     `json:"site_value"`
	TimeOffsetSeconds     *int       `json:"time_offset_seconds"`
	XrayMode              string     `json:"xray_mode"`
	TrafficUsedOffset     int64      `json:"traffic_used_offset"`
	SortOrder             int        `json:"sort_order"`
	TrafficStatsMode      string     `json:"traffic_stats_mode"`
	TrafficSource         string     `json:"traffic_source"`
	SystemRxCycle         int64      `json:"system_rx_cycle"`
	SystemTxCycle         int64      `json:"system_tx_cycle"`
	SystemLastSeenRx      int64      `json:"system_last_seen_rx"`
	SystemLastSeenTx      int64      `json:"system_last_seen_tx"`
	SystemBootTimeUnix    int64      `json:"system_boot_time_unix"`
	SystemTrafficUpdatedAt *time.Time `json:"system_traffic_updated_at"`
	DDNSEnabled           bool       `json:"ddns_enabled"`
	DDNSProviderID        int        `json:"ddns_provider_id"`
	DDNSLastSyncedAt      *time.Time `json:"ddns_last_synced_at"`
	DDNSLastError         string     `json:"ddns_last_error"`
	DDNSPending           bool       `json:"ddns_pending"`
	PullAddressV6         string     `json:"pull_address_v6"`
	DomainV6              string     `json:"domain_v6"`
	Ipv6Enabled           bool       `json:"ipv6_enabled"`
	ObservedIP            string     `json:"observed_ip"`
	ObservedIPV6          string     `json:"observed_ip_v6"`
	OfflineSince          *time.Time `json:"offline_since"`
	OfflineNotified       bool       `json:"offline_notified"`
	SameHostAsMaster      bool       `json:"same_host_as_master"`
	IncludeInTrafficStats bool       `json:"include_in_traffic_stats"`
	LockEntryIP           bool       `json:"lock_entry_ip"`
	PortRangeMin          int        `json:"port_range_min"`
	PortRangeMax          int        `json:"port_range_max"`
	LastTrafficResetAt    *time.Time `json:"last_traffic_reset_at"`
	Region                string     `json:"region"`
	RegionCountry         string     `json:"region_country"`
	RegionName            string     `json:"region_name"`
	RegionCity            string     `json:"region_city"`
	RenewalPrice          float64    `json:"renewal_price"`
	RenewalCycle          string     `json:"renewal_cycle"`
	RenewalCurrency       string     `json:"renewal_currency"`
	ProviderName          string     `json:"provider_name"`
	ProviderURL           string     `json:"provider_url"`
	TelecomPaidPeer       bool       `json:"telecom_paid_peer"`
	ProviderUpdatedAt     *time.Time `json:"provider_updated_at"`
	ExpiresAt             *time.Time `json:"expires_at"`
}

// remoteServersSchema is the core schema. We start with essential columns and use
// ensureRemoteServerColumn for the many incremental additions (matching mmwX's evolution).
const remoteServersSchema = `
CREATE TABLE IF NOT EXISTS remote_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    token TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'connected', 'offline')),
    last_heartbeat TIMESTAMP,
    ip_address TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateRemoteServers() error {
	if _, err := r.db.Exec(remoteServersSchema); err != nil {
		return fmt.Errorf("migrate remote_servers: %w", err)
	}
	// Incremental columns matching mmwX's 84-column table
	columns := []struct{ name, def string }{
		{"boot_time", "TIMESTAMP"},
		{"xray_boot_time", "TIMESTAMP"},
		{"boot_count", "INTEGER NOT NULL DEFAULT 0"},
		{"xray_boot_count", "INTEGER NOT NULL DEFAULT 0"},
		{"token_expires_at", "TIMESTAMP"},
		{"last_token_refresh", "TIMESTAMP"},
		{"connection_mode", "TEXT NOT NULL DEFAULT 'push'"},
		{"pull_address", "TEXT NOT NULL DEFAULT ''"},
		{"pull_port", "INTEGER NOT NULL DEFAULT 0"},
		{"pull_token", "TEXT NOT NULL DEFAULT ''"},
		{"last_pull_at", "TIMESTAMP"},
		{"push_fail_count", "INTEGER NOT NULL DEFAULT 0"},
		{"last_push_fail", "TIMESTAMP"},
		{"fallback_to_pull", "INTEGER NOT NULL DEFAULT 0"},
		{"fallback_at", "TIMESTAMP"},
		{"current_upload_speed", "INTEGER NOT NULL DEFAULT 0"},
		{"current_download_speed", "INTEGER NOT NULL DEFAULT 0"},
		{"speed_updated_at", "TIMESTAMP"},
		{"xray_running", "INTEGER NOT NULL DEFAULT 0"},
		{"xray_version", "TEXT NOT NULL DEFAULT ''"},
		{"xray_scanned_at", "TIMESTAMP"},
		{"listen_port", "INTEGER NOT NULL DEFAULT 0"},
		{"traffic_limit", "INTEGER NOT NULL DEFAULT 0"},
		{"traffic_reset_day", "INTEGER NOT NULL DEFAULT 0"},
		{"domain", "TEXT NOT NULL DEFAULT ''"},
		{"ip_address_v6", "TEXT NOT NULL DEFAULT ''"},
		{"warp_installed", "INTEGER NOT NULL DEFAULT 0"},
		{"agent_token", "TEXT NOT NULL DEFAULT ''"},
		{"agent_token_expires_at", "TIMESTAMP"},
		{"last_agent_token_refresh", "TIMESTAMP"},
		{"use_443", "INTEGER NOT NULL DEFAULT 0"},
		{"steal_mode", "TEXT NOT NULL DEFAULT 'tunnel'"},
		{"site_type", "TEXT NOT NULL DEFAULT ''"},
		{"site_value", "TEXT NOT NULL DEFAULT ''"},
		{"time_offset_seconds", "INTEGER"},
		{"xray_mode", "TEXT NOT NULL DEFAULT 'external'"},
		{"traffic_used_offset", "INTEGER NOT NULL DEFAULT 0"},
		{"sort_order", "INTEGER NOT NULL DEFAULT 0"},
		{"traffic_stats_mode", "TEXT NOT NULL DEFAULT 'both'"},
		{"traffic_source", "TEXT NOT NULL DEFAULT 'xray'"},
		{"system_rx_cycle", "INTEGER NOT NULL DEFAULT 0"},
		{"system_tx_cycle", "INTEGER NOT NULL DEFAULT 0"},
		{"system_last_seen_rx", "INTEGER NOT NULL DEFAULT 0"},
		{"system_last_seen_tx", "INTEGER NOT NULL DEFAULT 0"},
		{"system_boot_time_unix", "INTEGER NOT NULL DEFAULT 0"},
		{"system_traffic_updated_at", "TIMESTAMP"},
		{"ddns_enabled", "INTEGER NOT NULL DEFAULT 0"},
		{"ddns_provider_id", "INTEGER NOT NULL DEFAULT 0"},
		{"ddns_last_synced_at", "TIMESTAMP"},
		{"ddns_last_error", "TEXT NOT NULL DEFAULT ''"},
		{"ddns_pending", "INTEGER NOT NULL DEFAULT 0"},
		{"pull_address_v6", "TEXT NOT NULL DEFAULT ''"},
		{"domain_v6", "TEXT NOT NULL DEFAULT ''"},
		{"ipv6_enabled", "INTEGER NOT NULL DEFAULT 1"},
		{"observed_ip", "TEXT NOT NULL DEFAULT ''"},
		{"observed_ip_v6", "TEXT NOT NULL DEFAULT ''"},
		{"offline_since", "TIMESTAMP"},
		{"offline_notified", "INTEGER NOT NULL DEFAULT 0"},
		{"same_host_as_master", "INTEGER NOT NULL DEFAULT 0"},
		{"include_in_traffic_stats", "INTEGER NOT NULL DEFAULT 1"},
		{"lock_entry_ip", "INTEGER NOT NULL DEFAULT 0"},
		{"port_range_min", "INTEGER NOT NULL DEFAULT 0"},
		{"port_range_max", "INTEGER NOT NULL DEFAULT 0"},
		{"last_traffic_reset_at", "TIMESTAMP"},
		{"region", "TEXT NOT NULL DEFAULT ''"},
		{"region_country", "TEXT NOT NULL DEFAULT ''"},
		{"region_name", "TEXT NOT NULL DEFAULT ''"},
		{"region_city", "TEXT NOT NULL DEFAULT ''"},
		{"renewal_price", "NUMERIC NOT NULL DEFAULT 0"},
		{"renewal_cycle", "TEXT NOT NULL DEFAULT 'month'"},
		{"renewal_currency", "TEXT NOT NULL DEFAULT 'CNY'"},
		{"provider_name", "TEXT NOT NULL DEFAULT ''"},
		{"provider_url", "TEXT NOT NULL DEFAULT ''"},
		{"telecom_paid_peer", "INTEGER NOT NULL DEFAULT 0"},
		{"provider_updated_at", "TIMESTAMP"},
		{"expires_at", "TIMESTAMP"},
	}
	for _, col := range columns {
		if err := r.ensureRemoteServerColumn(col.name, col.def); err != nil {
			return err
		}
	}
	return nil
}

func (r *TrafficRepository) ensureRemoteServerColumn(name, definition string) error {
	rows, err := r.db.Query(`PRAGMA table_info(remote_servers)`)
	if err != nil {
		return fmt.Errorf("remote_servers table info: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var colName, colType string
		var notNull int
		var defaultVal sql.NullString
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &defaultVal, &pk); err != nil {
			return fmt.Errorf("scan table info: %w", err)
		}
		if equalFold(colName, name) {
			return nil
		}
	}
	alter := fmt.Sprintf("ALTER TABLE remote_servers ADD COLUMN %s %s", name, definition)
	if _, err := r.db.Exec(alter); err != nil {
		return fmt.Errorf("add column %s to remote_servers: %w", name, err)
	}
	return nil
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if toLowerByte(a[i]) != toLowerByte(b[i]) {
			return false
		}
	}
	return true
}

func toLowerByte(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// --- ServerReturnRoute ---

type ServerReturnRoute struct {
	ServerID  int64     `json:"server_id"`
	Carrier   string    `json:"carrier"`
	Region    string    `json:"region"`
	RouteType string    `json:"route_type"`
	EntryIP   string    `json:"entry_ip"`
	EntryASN  string    `json:"entry_asn"`
	Reason    string    `json:"reason"`
	TestedAt  time.Time `json:"tested_at"`
}

const serverReturnRoutesSchema = `
CREATE TABLE IF NOT EXISTS server_return_routes (
    server_id  INTEGER NOT NULL,
    carrier    TEXT NOT NULL,
    region     TEXT NOT NULL DEFAULT '',
    route_type TEXT NOT NULL DEFAULT 'Unknown',
    entry_ip   TEXT NOT NULL DEFAULT '',
    entry_asn  TEXT NOT NULL DEFAULT '',
    reason     TEXT NOT NULL DEFAULT '',
    tested_at  TIMESTAMP NOT NULL,
    PRIMARY KEY (server_id, carrier)
);
`

func (r *TrafficRepository) migrateServerReturnRoutes() error {
	_, err := r.db.Exec(serverReturnRoutesSchema)
	return err
}

// --- ServerSystemTrafficSnapshot ---

type ServerSystemTrafficSnapshot struct {
	ID        int64  `json:"id"`
	ServerID  int64  `json:"server_id"`
	Date      string `json:"date"`
	RxCycle   int64  `json:"rx_cycle"`
	TxCycle   int64  `json:"tx_cycle"`
	CreatedAt time.Time `json:"created_at"`
}

const serverSystemTrafficSnapshotsSchema = `
CREATE TABLE IF NOT EXISTS server_system_traffic_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    date TEXT NOT NULL,
    rx_cycle INTEGER NOT NULL DEFAULT 0,
    tx_cycle INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, date)
);
`

func (r *TrafficRepository) migrateServerSystemTrafficSnapshots() error {
	_, err := r.db.Exec(serverSystemTrafficSnapshotsSchema)
	return err
}

// --- ServerXrayConfigSnapshot ---

type ServerXrayConfigSnapshot struct {
	ID         int64     `json:"id"`
	ServerID   int64     `json:"server_id"`
	ConfigJSON string    `json:"config_json"`
	ConfigHash string    `json:"config_hash"`
	Source     string    `json:"source"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

const serverXrayConfigSnapshotsSchema = `
CREATE TABLE IF NOT EXISTS server_xray_config_snapshots (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id   INTEGER NOT NULL,
    config_json TEXT    NOT NULL,
    config_hash TEXT    NOT NULL,
    source      TEXT    NOT NULL,
    status      TEXT    NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (server_id) REFERENCES remote_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateServerXrayConfigSnapshots() error {
	_, err := r.db.Exec(serverXrayConfigSnapshotsSchema)
	return err
}

// --- NodeReachability ---

type NodeReachability struct {
	NodeID           int64     `json:"node_id"`
	Reachable        bool      `json:"reachable"`
	ConsecutiveFail  int       `json:"consecutive_fail"`
	Since            time.Time `json:"since"`
	AnnouncedBlocked bool      `json:"announced_blocked"`
}

const nodeReachabilitySchema = `
CREATE TABLE IF NOT EXISTS node_reachability (
    node_id INTEGER PRIMARY KEY,
    reachable INTEGER NOT NULL DEFAULT 1,
    consecutive_fail INTEGER NOT NULL DEFAULT 0,
    since TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    announced_blocked INTEGER NOT NULL DEFAULT 0
);
`

func (r *TrafficRepository) migrateNodeReachability() error {
	_, err := r.db.Exec(nodeReachabilitySchema)
	return err
}

// --- CRUD for RemoteServer ---

var ErrRemoteServerNotFound = errors.New("remote server not found")

func (r *TrafficRepository) CreateRemoteServer(ctx context.Context, s *RemoteServer) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO remote_servers (name, token, status, ip_address) VALUES (?, ?, ?, ?)`,
		s.Name, s.Token, s.Status, s.IPAddress)
	if err != nil {
		return 0, fmt.Errorf("create remote server: %w", err)
	}
	return res.LastInsertId()
}

func (r *TrafficRepository) ListRemoteServers(ctx context.Context) ([]RemoteServer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, status, COALESCE(ip_address,''), created_at, updated_at FROM remote_servers ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list remote servers: %w", err)
	}
	defer rows.Close()
	var out []RemoteServer
	for rows.Next() {
		var s RemoteServer
		if err := rows.Scan(&s.ID, &s.Name, &s.Status, &s.IPAddress, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) GetRemoteServer(ctx context.Context, id int64) (RemoteServer, error) {
	var s RemoteServer
	err := r.db.QueryRowContext(ctx, `SELECT id, name, status, COALESCE(ip_address,''), created_at, updated_at FROM remote_servers WHERE id = ?`, id).
		Scan(&s.ID, &s.Name, &s.Status, &s.IPAddress, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrRemoteServerNotFound
	}
	return s, err
}

func (r *TrafficRepository) GetRemoteServerByToken(ctx context.Context, token string) (RemoteServer, error) {
	var s RemoteServer
	err := r.db.QueryRowContext(ctx, `SELECT id, name, token, status, COALESCE(ip_address,''), created_at, updated_at FROM remote_servers WHERE token = ?`, token).
		Scan(&s.ID, &s.Name, &s.Token, &s.Status, &s.IPAddress, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrRemoteServerNotFound
	}
	return s, err
}

func (r *TrafficRepository) UpdateRemoteServerStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE remote_servers SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, id)
	return err
}

func (r *TrafficRepository) UpdateRemoteServerHeartbeat(ctx context.Context, id int64, ip string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE remote_servers SET last_heartbeat = CURRENT_TIMESTAMP, ip_address = ?, status = 'connected', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, ip, id)
	return err
}

func (r *TrafficRepository) DeleteRemoteServer(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM remote_servers WHERE id = ?`, id)
	return err
}
