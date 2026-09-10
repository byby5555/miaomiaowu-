package storage

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// --- ForwardChain ---

type ForwardChain struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	PortRangeStart int      `json:"port_range_start"`
	PortRangeEnd   int      `json:"port_range_end"`
	DNSDomain      string    `json:"dns_domain"`
	DNSDomainV6    string    `json:"dns_domain_v6"`
	DNSProviderID  int       `json:"dns_provider_id"`
}

const forwardChainsSchema = `
CREATE TABLE IF NOT EXISTS forward_chains (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    port_range_start INTEGER NOT NULL DEFAULT 0,
    port_range_end INTEGER NOT NULL DEFAULT 0,
    dns_domain TEXT NOT NULL DEFAULT '',
    dns_domain_v6 TEXT NOT NULL DEFAULT '',
    dns_provider_id INTEGER NOT NULL DEFAULT 0
);
`

func (r *TrafficRepository) migrateForwardChains() error {
	_, err := r.db.Exec(forwardChainsSchema)
	return err
}

// --- ForwardChainHop ---

type ForwardChainHop struct {
	ChainID int64 `json:"chain_id"`
	Seq     int   `json:"seq"`
	GroupID int64 `json:"group_id"`
}

const forwardChainHopsSchema = `
CREATE TABLE IF NOT EXISTS forward_chain_hops (
    chain_id INTEGER NOT NULL,
    seq INTEGER NOT NULL,
    group_id INTEGER NOT NULL,
    PRIMARY KEY(chain_id, seq)
);
`

func (r *TrafficRepository) migrateForwardChainHops() error {
	_, err := r.db.Exec(forwardChainHopsSchema)
	return err
}

// --- ForwardChainBranch ---

type ForwardChainBranch struct {
	ChainID     int64 `json:"chain_id"`
	HopSeq      int   `json:"hop_seq"`
	ServerID    int64 `json:"server_id"`
	Seq         int   `json:"seq"`
	ViaGroupID  int64 `json:"via_group_id"`
}

const forwardChainBranchesSchema = `
CREATE TABLE IF NOT EXISTS forward_chain_branches (
    chain_id     INTEGER NOT NULL,
    hop_seq      INTEGER NOT NULL,
    server_id    INTEGER NOT NULL,
    seq          INTEGER NOT NULL,
    via_group_id INTEGER NOT NULL,
    PRIMARY KEY(chain_id, hop_seq, server_id, seq)
);
`

func (r *TrafficRepository) migrateForwardChainBranches() error {
	_, err := r.db.Exec(forwardChainBranchesSchema)
	return err
}

// --- ForwardChainNode ---

type ForwardChainNode struct {
	NodeID             int64     `json:"node_id"`
	ChainID            int64     `json:"chain_id"`
	Port               int       `json:"port"`
	CreatedAt          time.Time `json:"created_at"`
	TerminusAddr       string    `json:"terminus_addr"`
	Protocol           string    `json:"protocol"`
	PinnedExitServerID int64     `json:"pinned_exit_server_id"`
	RateLimitMbps      float64   `json:"rate_limit_mbps"`
	ConnLimit          int       `json:"conn_limit"`
	IPLimit            int       `json:"ip_limit"`
	OwnerUsername      string    `json:"owner_username"`
}

const forwardChainNodesSchema = `
CREATE TABLE IF NOT EXISTS forward_chain_nodes (
    node_id INTEGER PRIMARY KEY,
    chain_id INTEGER NOT NULL,
    port INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    terminus_addr TEXT NOT NULL DEFAULT '',
    protocol TEXT NOT NULL DEFAULT 'tcp',
    pinned_exit_server_id INTEGER NOT NULL DEFAULT 0,
    rate_limit_mbps REAL NOT NULL DEFAULT 0,
    conn_limit INTEGER NOT NULL DEFAULT 0,
    ip_limit INTEGER NOT NULL DEFAULT 0,
    owner_username TEXT NOT NULL DEFAULT ''
);
`

func (r *TrafficRepository) migrateForwardChainNodes() error {
	_, err := r.db.Exec(forwardChainNodesSchema)
	return err
}

// --- ForwardGroup ---

type ForwardGroup struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	BalanceStrategy   string    `json:"balance_strategy"` // round_robin | weighted | least_conn
	DNSDomain         string    `json:"dns_domain"`
	DNSProviderID     int       `json:"dns_provider_id"`
	FailoverEnabled   bool      `json:"failover_enabled"`
	OfflineMsThreshold int      `json:"offline_ms_threshold"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	TrafficMultiplier float64   `json:"traffic_multiplier"`
}

const forwardGroupsSchema = `
CREATE TABLE IF NOT EXISTS forward_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    balance_strategy TEXT NOT NULL DEFAULT 'round_robin',
    dns_domain TEXT NOT NULL DEFAULT '',
    dns_provider_id INTEGER NOT NULL DEFAULT 0,
    failover_enabled INTEGER NOT NULL DEFAULT 1,
    offline_ms_threshold INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    traffic_multiplier REAL NOT NULL DEFAULT 1
);
`

func (r *TrafficRepository) migrateForwardGroups() error {
	_, err := r.db.Exec(forwardGroupsSchema)
	return err
}

// --- ForwardGroupMember ---

type ForwardGroupMember struct {
	GroupID  int64 `json:"group_id"`
	ServerID int64 `json:"server_id"`
	Weight   int   `json:"weight"`
	Seq      int   `json:"seq"`
}

const forwardGroupMembersSchema = `
CREATE TABLE IF NOT EXISTS forward_group_members (
    group_id INTEGER NOT NULL,
    server_id INTEGER NOT NULL,
    weight INTEGER NOT NULL DEFAULT 1,
    seq INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY(group_id, server_id)
);
`

func (r *TrafficRepository) migrateForwardGroupMembers() error {
	_, err := r.db.Exec(forwardGroupMembersSchema)
	return err
}

// --- ForwardDailyTraffic ---

type ForwardDailyTraffic struct {
	ChainID   int64     `json:"chain_id"`
	ServerID  int64     `json:"server_id"`
	Date      string    `json:"date"`
	BytesUp   int64     `json:"bytes_up"`
	BytesDown int64     `json:"bytes_down"`
	UpdatedAt time.Time `json:"updated_at"`
}

const forwardDailyTrafficSchema = `
CREATE TABLE IF NOT EXISTS forward_daily_traffic (
    chain_id INTEGER NOT NULL,
    server_id INTEGER NOT NULL,
    date TEXT NOT NULL,
    bytes_up BIGINT NOT NULL DEFAULT 0,
    bytes_down BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(chain_id, server_id, date)
);
`

func (r *TrafficRepository) migrateForwardDailyTraffic() error {
	_, err := r.db.Exec(forwardDailyTrafficSchema)
	return err
}

// --- ForwardHopMetric ---

type ForwardHopMetric struct {
	ID           int64     `json:"id"`
	ServerID     int64     `json:"server_id"`
	RuleID       string    `json:"rule_id"`
	UpstreamAddr string    `json:"upstream_addr"`
	Healthy      bool      `json:"healthy"`
	RttMs        int       `json:"rtt_ms"`
	LossPermille int       `json:"loss_permille"`
	JitterMs     int       `json:"jitter_ms"`
	BytesUp      int64     `json:"bytes_up"`
	BytesDown    int64     `json:"bytes_down"`
	At           time.Time `json:"at"`
}

const forwardHopMetricsSchema = `
CREATE TABLE IF NOT EXISTS forward_hop_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    rule_id TEXT NOT NULL,
    upstream_addr TEXT NOT NULL,
    healthy INTEGER NOT NULL DEFAULT 0,
    rtt_ms INTEGER NOT NULL DEFAULT 0,
    loss_permille INTEGER NOT NULL DEFAULT 0,
    jitter_ms INTEGER NOT NULL DEFAULT 0,
    bytes_up INTEGER NOT NULL DEFAULT 0,
    bytes_down INTEGER NOT NULL DEFAULT 0,
    at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateForwardHopMetrics() error {
	_, err := r.db.Exec(forwardHopMetricsSchema)
	return err
}

// --- WireGuard ---

type WGDevice struct {
	ID               int64     `json:"id"`
	ServerID         int64     `json:"server_id"`
	InboundTag       string    `json:"inbound_tag"`
	IPv4CIDR         string    `json:"ipv4_cidr"`
	IPv6CIDR         string    `json:"ipv6_cidr"`
	FirstIndex       int       `json:"first_index"`
	LastIndex        int       `json:"last_index"`
	ServerPrivateKey string    `json:"-"`
	ServerPublicKey  string    `json:"server_public_key"`
	ProbePrivateKey  string    `json:"-"`
	ProbePublicKey   string    `json:"probe_public_key"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

const wgDevicesSchema = `
CREATE TABLE IF NOT EXISTS wg_devices (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    inbound_tag TEXT NOT NULL,
    ipv4_cidr TEXT NOT NULL,
    ipv6_cidr TEXT NOT NULL,
    first_index INTEGER NOT NULL,
    last_index INTEGER NOT NULL,
    server_private_key TEXT NOT NULL,
    server_public_key TEXT NOT NULL,
    probe_private_key TEXT NOT NULL DEFAULT '',
    probe_public_key TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, inbound_tag),
    UNIQUE(server_id, ipv4_cidr)
);
`

func (r *TrafficRepository) migrateWGDevices() error {
	_, err := r.db.Exec(wgDevicesSchema)
	return err
}

type WGLease struct {
	ID           int64      `json:"id"`
	DeviceID     int64      `json:"device_id"`
	HostIndex    int        `json:"host_index"`
	Username     string     `json:"username"`
	AssignmentID int64      `json:"assignment_id"`
	Email        string     `json:"email"`
	PrivateKey   string     `json:"-"`
	PublicKey    string     `json:"public_key"`
	IPv4         string     `json:"ipv4"`
	IPv6         string     `json:"ipv6"`
	ReleasedAt   *time.Time `json:"released_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

const wgLeasesSchema = `
CREATE TABLE IF NOT EXISTS wg_leases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER NOT NULL,
    host_index INTEGER NOT NULL,
    username TEXT NOT NULL,
    assignment_id INTEGER NOT NULL DEFAULT 0,
    email TEXT NOT NULL,
    private_key TEXT NOT NULL,
    public_key TEXT NOT NULL,
    ipv4 TEXT NOT NULL,
    ipv6 TEXT NOT NULL,
    released_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(device_id, host_index),
    UNIQUE(device_id, email)
);
`

func (r *TrafficRepository) migrateWGLeases() error {
	_, err := r.db.Exec(wgLeasesSchema)
	return err
}

// --- Federation & Sharing ---

type FederatedServer struct {
	ServerID  int64     `json:"server_id"`
	OwnerURL  string    `json:"owner_url"`
	ShareToken string   `json:"-"`
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"created_at"`
}

const federatedServersSchema = `
CREATE TABLE IF NOT EXISTS federated_servers (
    server_id INTEGER PRIMARY KEY,
    owner_url TEXT NOT NULL,
    share_token TEXT NOT NULL,
    prefix TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateFederatedServers() error {
	_, err := r.db.Exec(federatedServersSchema)
	return err
}

type SharedServer struct {
	ID               int64      `json:"id"`
	ServerID         int64      `json:"server_id"`
	TokenHash        string     `json:"-"`
	Label            string     `json:"label"`
	CreatedAt        time.Time  `json:"created_at"`
	RevokedAt        *time.Time `json:"revoked_at"`
	AllowManageXray  bool       `json:"allow_manage_xray"`
}

const sharedServersSchema = `
CREATE TABLE IF NOT EXISTS shared_servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    label TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMP,
    allow_manage_xray INTEGER NOT NULL DEFAULT 0
);
`

func (r *TrafficRepository) migrateSharedServers() error {
	_, err := r.db.Exec(sharedServersSchema)
	return err
}

// --- Announcements ---

type Announcement struct {
	ID            int64      `json:"id"`
	Type          string     `json:"type"` // general | node | maintenance
	Title         string     `json:"title"`
	Body          string     `json:"body"`
	NodeID        int64      `json:"node_id"`
	ViaBot        bool       `json:"via_bot"`
	ViaMiniapp    bool       `json:"via_miniapp"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     *time.Time `json:"expires_at"`
	BotDeliveredAt *time.Time `json:"bot_delivered_at"`
}

const announcementsSchema = `
CREATE TABLE IF NOT EXISTS announcements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT NOT NULL DEFAULT 'general',
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    node_id INTEGER NOT NULL DEFAULT 0,
    via_bot INTEGER NOT NULL DEFAULT 1,
    via_miniapp INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    bot_delivered_at TIMESTAMP
);
`

func (r *TrafficRepository) migrateAnnouncements() error {
	_, err := r.db.Exec(announcementsSchema)
	return err
}

var ErrAnnouncementNotFound = errors.New("announcement not found")

func (r *TrafficRepository) CreateAnnouncement(ctx context.Context, a *Announcement) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO announcements (type, title, body, node_id, via_bot, via_miniapp, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.Type, a.Title, a.Body, a.NodeID, boolToInt(a.ViaBot), boolToInt(a.ViaMiniapp), a.ExpiresAt)
	if err != nil {
		return 0, fmt.Errorf("create announcement: %w", err)
	}
	return res.LastInsertId()
}

func (r *TrafficRepository) ListAnnouncements(ctx context.Context) ([]Announcement, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, type, title, body, node_id, via_bot, via_miniapp, created_at, expires_at, bot_delivered_at FROM announcements ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Announcement
	for rows.Next() {
		var a Announcement
		var viaBot, viaMiniapp int
		if err := rows.Scan(&a.ID, &a.Type, &a.Title, &a.Body, &a.NodeID, &viaBot, &viaMiniapp, &a.CreatedAt, &a.ExpiresAt, &a.BotDeliveredAt); err != nil {
			return nil, err
		}
		a.ViaBot = viaBot != 0
		a.ViaMiniapp = viaMiniapp != 0
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) DeleteAnnouncement(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM announcements WHERE id = ?`, id)
	return err
}

// --- RoutingRulePreset ---

type RoutingRulePreset struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	RuleJSON  string    `json:"rule_json"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

const routingRulePresetsSchema = `
CREATE TABLE IF NOT EXISTS routing_rule_presets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    name TEXT NOT NULL,
    rule_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE,
    UNIQUE(username, rule_json)
);
`

func (r *TrafficRepository) migrateRoutingRulePresets() error {
	_, err := r.db.Exec(routingRulePresetsSchema)
	return err
}

// --- TGAudit ---

type TGAudit struct {
	ID       int64     `json:"id"`
	TGID     *int64    `json:"tg_id"`
	Username string    `json:"username"`
	Action   string    `json:"action"`
	Detail   string    `json:"detail"`
	At       time.Time `json:"at"`
}

const tgAuditSchema = `
CREATE TABLE IF NOT EXISTS tg_audit (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    tg_id     INTEGER,
    username  TEXT NOT NULL DEFAULT '',
    action    TEXT NOT NULL,
    detail    TEXT NOT NULL DEFAULT '',
    at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateTGAudit() error {
	_, err := r.db.Exec(tgAuditSchema)
	return err
}

// --- PackageAssignment sub-tables ---

const packageAssignmentInboundConfigsSchema = `
CREATE TABLE IF NOT EXISTS package_assignment_inbound_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assignment_id INTEGER NOT NULL,
    username TEXT NOT NULL,
    server_id INTEGER NOT NULL,
    inbound_tag TEXT NOT NULL,
    protocol TEXT NOT NULL,
    email TEXT NOT NULL,
    credential_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(assignment_id,server_id,inbound_tag),
    FOREIGN KEY(assignment_id) REFERENCES user_package_assignments(id) ON DELETE CASCADE,
    FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY(server_id) REFERENCES remote_servers(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migratePackageAssignmentInboundConfigs() error {
	_, err := r.db.Exec(packageAssignmentInboundConfigsSchema)
	return err
}

const packageAssignmentSubaccountsSchema = `
CREATE TABLE IF NOT EXISTS package_assignment_subaccounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    assignment_id INTEGER NOT NULL,
    username TEXT NOT NULL,
    routed_node_id INTEGER NOT NULL,
    email TEXT NOT NULL,
    credential_json TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(assignment_id,routed_node_id),
    UNIQUE(routed_node_id,email),
    FOREIGN KEY(assignment_id) REFERENCES user_package_assignments(id) ON DELETE CASCADE,
    FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migratePackageAssignmentSubaccounts() error {
	_, err := r.db.Exec(packageAssignmentSubaccountsSchema)
	return err
}

const packageNodeTrafficSuspensionsSchema = `
CREATE TABLE IF NOT EXISTS package_node_traffic_suspensions (
    username TEXT NOT NULL,
    package_id INTEGER NOT NULL,
    node_id INTEGER NOT NULL,
    kind TEXT NOT NULL,
    credential_json TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(username, package_id, node_id, kind)
);
`

func (r *TrafficRepository) migratePackageNodeTrafficSuspensions() error {
	_, err := r.db.Exec(packageNodeTrafficSuspensionsSchema)
	return err
}

const packageUserNodeTrafficBaselinesSchema = `
CREATE TABLE IF NOT EXISTS package_user_node_traffic_baselines (
    username TEXT NOT NULL,
    package_id INTEGER NOT NULL,
    node_id INTEGER NOT NULL,
    baseline REAL NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY(username, package_id, node_id)
);
`

func (r *TrafficRepository) migratePackageUserNodeTrafficBaselines() error {
	_, err := r.db.Exec(packageUserNodeTrafficBaselinesSchema)
	return err
}

// --- Traffic daily tables (12 tables matching mmwX) ---

const trafficDailyNodesSchema = `
CREATE TABLE IF NOT EXISTS traffic_daily_nodes (
    server_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    type TEXT NOT NULL,
    date TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (server_id, tag, type, date),
    FOREIGN KEY (server_id) REFERENCES remote_servers(id) ON DELETE CASCADE
);
`

const trafficDailyUsersSchema = `
CREATE TABLE IF NOT EXISTS traffic_daily_users (
    server_id INTEGER NOT NULL,
    username TEXT NOT NULL,
    date TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (server_id, username, date),
    FOREIGN KEY (server_id) REFERENCES remote_servers(id) ON DELETE CASCADE
);
`

const trafficDailySystemServersSchema = `
CREATE TABLE IF NOT EXISTS traffic_daily_system_servers (
    server_id INTEGER NOT NULL,
    date TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (server_id, date),
    FOREIGN KEY (server_id) REFERENCES remote_servers(id) ON DELETE CASCADE
);
`

const trafficDailyUserNodesSchema = `
CREATE TABLE IF NOT EXISTS traffic_daily_user_nodes (
    server_id INTEGER NOT NULL,
    node_id INTEGER NOT NULL,
    username TEXT NOT NULL,
    date TEXT NOT NULL,
    uplink REAL NOT NULL DEFAULT 0,
    downlink REAL NOT NULL DEFAULT 0,
    weighted_uplink REAL NOT NULL DEFAULT 0,
    weighted_downlink REAL NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (server_id, node_id, username, date),
    FOREIGN KEY (server_id) REFERENCES remote_servers(id) ON DELETE CASCADE
);
`

const trafficDailyUserEmailsSchema = `
CREATE TABLE IF NOT EXISTS traffic_daily_user_emails (
    server_id INTEGER NOT NULL,
    email TEXT NOT NULL,
    attributed_username TEXT NOT NULL DEFAULT '',
    date TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    weighted_uplink REAL NOT NULL DEFAULT 0,
    weighted_downlink REAL NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (server_id, email, attributed_username, date),
    FOREIGN KEY (server_id) REFERENCES remote_servers(id) ON DELETE CASCADE
);
`

const trafficDailyExternalSubscriptionsSchema = `
CREATE TABLE IF NOT EXISTS traffic_daily_external_subscriptions (
    external_subscription_id INTEGER NOT NULL,
    date TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (external_subscription_id, date),
    FOREIGN KEY (external_subscription_id) REFERENCES external_subscriptions(id) ON DELETE CASCADE
);
`

const trafficDailyUsersArchivedSchema = `
CREATE TABLE IF NOT EXISTS traffic_daily_users_archived (
    username TEXT NOT NULL,
    date TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (username, date)
);
`

const trafficDailyIncompleteDatesSchema = `
CREATE TABLE IF NOT EXISTS traffic_daily_incomplete_dates(
    date TEXT PRIMARY KEY,
    reason TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

const trafficDailyMetaSchema = `
CREATE TABLE IF NOT EXISTS traffic_daily_meta(
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

const trafficThresholdNotifiedSchema = `
CREATE TABLE IF NOT EXISTS traffic_threshold_notified (
    server_id INTEGER PRIMARY KEY,
    notified_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

const nodeTrafficSnapshotsSchema = `
CREATE TABLE IF NOT EXISTS node_traffic_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    tag TEXT NOT NULL,
    type TEXT NOT NULL DEFAULT 'inbound',
    date TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, tag, type, date)
);
`

const userTrafficSnapshotsSchema = `
CREATE TABLE IF NOT EXISTS user_traffic_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    username TEXT NOT NULL,
    date TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, username, date)
);
`

const userEmailTrafficSnapshotsSchema = `
CREATE TABLE IF NOT EXISTS user_email_traffic_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    email TEXT NOT NULL,
    date TEXT NOT NULL,
    uplink INTEGER NOT NULL DEFAULT 0,
    downlink INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, email, date)
);
`

func (r *TrafficRepository) migrateTrafficDailyTables() error {
	schemas := []string{
		trafficDailyNodesSchema,
		trafficDailyUsersSchema,
		trafficDailySystemServersSchema,
		trafficDailyUserNodesSchema,
		trafficDailyUserEmailsSchema,
		trafficDailyExternalSubscriptionsSchema,
		trafficDailyUsersArchivedSchema,
		trafficDailyIncompleteDatesSchema,
		trafficDailyMetaSchema,
		trafficThresholdNotifiedSchema,
		nodeTrafficSnapshotsSchema,
		userTrafficSnapshotsSchema,
		userEmailTrafficSnapshotsSchema,
	}
	for _, s := range schemas {
		if _, err := r.db.Exec(s); err != nil {
			return fmt.Errorf("migrate traffic daily table: %w", err)
		}
	}
	return nil
}
