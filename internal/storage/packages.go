package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Package represents a subscription package with traffic, speed, and node limits.
type Package struct {
	ID                       int64     `json:"id"`
	Name                     string    `json:"name"`
	Description              string    `json:"description"`
	TrafficLimitBytes        int64     `json:"traffic_limit_bytes"`
	CycleDays                int       `json:"cycle_days"`
	IsReset                  bool      `json:"is_reset"`
	ResetDay                 int       `json:"reset_day"`
	Nodes                    string    `json:"nodes"` // JSON array
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
	ShortCode                string    `json:"short_code"`
	NodeMultipliers          string    `json:"node_multipliers"` // JSON object
	SpeedLimitMbps           float64   `json:"speed_limit_mbps"`
	DeviceLimit              int       `json:"device_limit"`
	AutoSpeedLimitJSON       string    `json:"auto_speed_limit_json"`
	TrafficMode              string    `json:"traffic_mode"` // oneway | bidirectional
	TemplateFilename         string    `json:"template_filename"`
	NodeSpeedLimits          string    `json:"node_speed_limits"` // JSON object
	NodeDeviceLimits         string    `json:"node_device_limits"` // JSON object
	NodeNameOverrides        string    `json:"node_name_overrides"` // JSON object
	NodeNameOverrideEnabled  bool      `json:"node_name_override_enabled"`
	SurgeTemplateFilename    string    `json:"surge_template_filename"`
	ForwardRuleLimit         int       `json:"forward_rule_limit"`
	ForwardPortLimit         int       `json:"forward_port_limit"`
	ForwardSpeedMbps         float64   `json:"forward_speed_mbps"`
	ForwardConnLimit         int       `json:"forward_conn_limit"`
	ForwardChains            string    `json:"forward_chains"` // JSON array
	NodeTrafficLimits        string    `json:"node_traffic_limits"` // JSON object
}

const packagesSchema = `
CREATE TABLE IF NOT EXISTS packages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    traffic_limit_bytes INTEGER NOT NULL DEFAULT 0,
    cycle_days INTEGER NOT NULL DEFAULT 30,
    is_reset INTEGER NOT NULL DEFAULT 0,
    reset_day INTEGER NOT NULL DEFAULT 1,
    nodes TEXT DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migratePackages() error {
	if _, err := r.db.Exec(packagesSchema); err != nil {
		return fmt.Errorf("migrate packages: %w", err)
	}
	cols := []struct{ name, def string }{
		{"short_code", "TEXT DEFAULT ''"},
		{"node_multipliers", "TEXT DEFAULT '{}'"},
		{"speed_limit_mbps", "REAL NOT NULL DEFAULT 0"},
		{"device_limit", "INTEGER NOT NULL DEFAULT 0"},
		{"auto_speed_limit_json", "TEXT DEFAULT ''"},
		{"traffic_mode", "TEXT NOT NULL DEFAULT 'oneway'"},
		{"template_filename", "TEXT NOT NULL DEFAULT ''"},
		{"node_speed_limits", "TEXT DEFAULT '{}'"},
		{"node_device_limits", "TEXT DEFAULT '{}'"},
		{"node_name_overrides", "TEXT DEFAULT '{}'"},
		{"node_name_override_enabled", "INTEGER NOT NULL DEFAULT 0"},
		{"surge_template_filename", "TEXT NOT NULL DEFAULT ''"},
		{"forward_rule_limit", "INTEGER NOT NULL DEFAULT 0"},
		{"forward_port_limit", "INTEGER NOT NULL DEFAULT 0"},
		{"forward_speed_mbps", "REAL NOT NULL DEFAULT 0"},
		{"forward_conn_limit", "INTEGER NOT NULL DEFAULT 0"},
		{"forward_chains", "TEXT DEFAULT '[]'"},
		{"node_traffic_limits", "TEXT DEFAULT '{}'"},
	}
	for _, c := range cols {
		if err := r.ensurePackageColumn(c.name, c.def); err != nil {
			return err
		}
	}
	return nil
}

func (r *TrafficRepository) ensurePackageColumn(name, def string) error {
	rows, err := r.db.Query(`PRAGMA table_info(packages)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var colName, colType string
		var notNull int
		var defaultVal sql.NullString
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &defaultVal, &pk); err != nil {
			return err
		}
		if equalFold(colName, name) {
			return nil
		}
	}
	_, err = r.db.Exec(fmt.Sprintf("ALTER TABLE packages ADD COLUMN %s %s", name, def))
	return err
}

var ErrPackageNotFound = errors.New("package not found")

func (r *TrafficRepository) CreatePackage(ctx context.Context, p *Package) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO packages (name, description, traffic_limit_bytes, cycle_days, is_reset, reset_day, nodes, short_code, speed_limit_mbps, device_limit, traffic_mode, template_filename) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Description, p.TrafficLimitBytes, p.CycleDays, boolToInt(p.IsReset), p.ResetDay, p.Nodes, p.ShortCode, p.SpeedLimitMbps, p.DeviceLimit, p.TrafficMode, p.TemplateFilename)
	if err != nil {
		return 0, fmt.Errorf("create package: %w", err)
	}
	return res.LastInsertId()
}

func (r *TrafficRepository) ListPackages(ctx context.Context) ([]Package, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, COALESCE(description,''), traffic_limit_bytes, cycle_days, is_reset, reset_day, COALESCE(nodes,'[]'), created_at, updated_at, COALESCE(short_code,''), COALESCE(speed_limit_mbps,0), device_limit, COALESCE(traffic_mode,'oneway'), COALESCE(template_filename,'') FROM packages ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Package
	for rows.Next() {
		var p Package
		var isReset int
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.TrafficLimitBytes, &p.CycleDays, &isReset, &p.ResetDay, &p.Nodes, &p.CreatedAt, &p.UpdatedAt, &p.ShortCode, &p.SpeedLimitMbps, &p.DeviceLimit, &p.TrafficMode, &p.TemplateFilename); err != nil {
			return nil, err
		}
		p.IsReset = isReset != 0
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) GetPackage(ctx context.Context, id int64) (Package, error) {
	var p Package
	var isReset int
	err := r.db.QueryRowContext(ctx, `SELECT id, name, COALESCE(description,''), traffic_limit_bytes, cycle_days, is_reset, reset_day, COALESCE(nodes,'[]'), created_at, updated_at, COALESCE(short_code,''), COALESCE(speed_limit_mbps,0), device_limit, COALESCE(traffic_mode,'oneway'), COALESCE(template_filename,'') FROM packages WHERE id = ?`, id).
		Scan(&p.ID, &p.Name, &p.Description, &p.TrafficLimitBytes, &p.CycleDays, &isReset, &p.ResetDay, &p.Nodes, &p.CreatedAt, &p.UpdatedAt, &p.ShortCode, &p.SpeedLimitMbps, &p.DeviceLimit, &p.TrafficMode, &p.TemplateFilename)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrPackageNotFound
	}
	p.IsReset = isReset != 0
	return p, err
}

func (r *TrafficRepository) UpdatePackage(ctx context.Context, p *Package) error {
	_, err := r.db.ExecContext(ctx, `UPDATE packages SET name=?, description=?, traffic_limit_bytes=?, cycle_days=?, is_reset=?, reset_day=?, nodes=?, speed_limit_mbps=?, device_limit=?, traffic_mode=?, template_filename=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		p.Name, p.Description, p.TrafficLimitBytes, p.CycleDays, boolToInt(p.IsReset), p.ResetDay, p.Nodes, p.SpeedLimitMbps, p.DeviceLimit, p.TrafficMode, p.TemplateFilename, p.ID)
	return err
}

func (r *TrafficRepository) DeletePackage(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM packages WHERE id = ?`, id)
	return err
}

// --- UserPackageAssignment ---

type UserPackageAssignment struct {
	ID                 int64      `json:"id"`
	Username           string     `json:"username"`
	PackageID          int64      `json:"package_id"`
	PackageStartDate   *time.Time `json:"package_start_date"`
	PackageEndDate     *time.Time `json:"package_end_date"`
	IsReset            bool       `json:"is_reset"`
	ResetDay           int        `json:"reset_day"`
	LastResetAt        *time.Time `json:"last_reset_at"`
	TrafficLimitOverride *int64   `json:"traffic_limit_override"`
	Status             string     `json:"status"` // active | expired | suspended
	IsPrimary          bool       `json:"is_primary"`
	ShortCode          string     `json:"short_code"`
	TrafficWarned80    bool       `json:"traffic_warned_80"`
	OverLimitEnforced  bool       `json:"over_limit_enforced"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

const userPackageAssignmentsSchema = `
CREATE TABLE IF NOT EXISTS user_package_assignments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    package_id INTEGER NOT NULL,
    package_start_date TIMESTAMP,
    package_end_date TIMESTAMP,
    is_reset INTEGER NOT NULL DEFAULT 0,
    reset_day INTEGER NOT NULL DEFAULT 1,
    last_reset_at TIMESTAMP,
    traffic_limit_override INTEGER,
    status TEXT NOT NULL DEFAULT 'active',
    is_primary INTEGER NOT NULL DEFAULT 0,
    short_code TEXT NOT NULL UNIQUE,
    traffic_warned_80 INTEGER NOT NULL DEFAULT 0,
    over_limit_enforced INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE,
    FOREIGN KEY(package_id) REFERENCES packages(id) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateUserPackageAssignments() error {
	_, err := r.db.Exec(userPackageAssignmentsSchema)
	return err
}

// --- InviteCode ---

type InviteCode struct {
	Code           string     `json:"code"`
	Kind           string     `json:"kind"` // new | bind
	BindUsername   string     `json:"bind_username"`
	CreatedBy      string     `json:"created_by"`
	PackageID      *int64     `json:"package_id"`
	MaxUses        int        `json:"max_uses"`
	UsedCount      int        `json:"used_count"`
	ExpiresAt      *time.Time `json:"expires_at"`
	Revoked        bool       `json:"revoked"`
	Remark         string     `json:"remark"`
	CreatedAt      time.Time  `json:"created_at"`
	DurationMonths int        `json:"duration_months"`
}

const inviteCodesSchema = `
CREATE TABLE IF NOT EXISTS invite_codes (
    code           TEXT PRIMARY KEY,
    kind           TEXT NOT NULL CHECK (kind IN ('new', 'bind')),
    bind_username  TEXT NOT NULL DEFAULT '',
    created_by     TEXT NOT NULL,
    package_id     INTEGER,
    max_uses       INTEGER NOT NULL DEFAULT 1,
    used_count     INTEGER NOT NULL DEFAULT 0,
    expires_at     TIMESTAMP,
    revoked        INTEGER NOT NULL DEFAULT 0,
    remark         TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    duration_months INTEGER NOT NULL DEFAULT 0
);
`

func (r *TrafficRepository) migrateInviteCodes() error {
	_, err := r.db.Exec(inviteCodesSchema)
	return err
}

// --- InviteCodeUse ---

type InviteCodeUse struct {
	Code     string    `json:"code"`
	Username string    `json:"username"`
	TGID     *int64    `json:"tg_id"`
	UsedAt   time.Time `json:"used_at"`
}

const inviteCodeUsesSchema = `
CREATE TABLE IF NOT EXISTS invite_code_uses (
    code       TEXT NOT NULL,
    username   TEXT NOT NULL,
    tg_id      INTEGER,
    used_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (code, username)
);
`

func (r *TrafficRepository) migrateInviteCodeUses() error {
	_, err := r.db.Exec(inviteCodeUsesSchema)
	return err
}

// --- RenewalRequest ---

type RenewalRequest struct {
	ID             int64      `json:"id"`
	RequestToken   string     `json:"request_token"`
	Username       string     `json:"username"`
	TelegramID     int64      `json:"telegram_id"`
	PackageID      int64      `json:"package_id"`
	PackageName    string     `json:"package_name"`
	PreviousEndDate *time.Time `json:"previous_end_date"`
	RenewDays      int        `json:"renew_days"`
	Passphrase     string     `json:"-"`
	Source         string     `json:"source"` // web | telegram
	Status         string     `json:"status"` // pending | approved | rejected
	ReviewedBy     int64      `json:"reviewed_by"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	NewEndDate     *time.Time `json:"new_end_date"`
	ErrorMessage   string     `json:"error_message"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

const renewalRequestsSchema = `
CREATE TABLE IF NOT EXISTS renewal_requests (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    request_token TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL,
    telegram_id INTEGER NOT NULL DEFAULT 0,
    package_id INTEGER NOT NULL,
    package_name TEXT NOT NULL DEFAULT '',
    previous_end_date TIMESTAMP,
    renew_days INTEGER NOT NULL,
    passphrase TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'web',
    status TEXT NOT NULL DEFAULT 'pending',
    reviewed_by INTEGER NOT NULL DEFAULT 0,
    reviewed_at TIMESTAMP,
    new_end_date TIMESTAMP,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateRenewalRequests() error {
	_, err := r.db.Exec(renewalRequestsSchema)
	return err
}

// --- UserTrafficRecord (per-user traffic totals per date) ---

type UserTrafficRecord struct {
	Username      string    `json:"username"`
	Date          string    `json:"date"`
	TotalLimit    int64     `json:"total_limit"`
	TotalUsed     int64     `json:"total_used"`
	TotalRemaining int64    `json:"total_remaining"`
	CreatedAt     time.Time `json:"created_at"`
}

const userTrafficRecordsSchema = `
CREATE TABLE IF NOT EXISTS user_traffic_records (
    username TEXT NOT NULL,
    date TEXT NOT NULL,
    total_limit INTEGER NOT NULL,
    total_used INTEGER NOT NULL,
    total_remaining INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (username, date)
);
`

func (r *TrafficRepository) migrateUserTrafficRecords() error {
	_, err := r.db.Exec(userTrafficRecordsSchema)
	return err
}

// --- UserTrafficCycleCarry ---

type UserTrafficCycleCarry struct {
	Username         string    `json:"username"`
	WeightedUplink   float64   `json:"weighted_uplink"`
	WeightedDownlink float64   `json:"weighted_downlink"`
	UpdatedAt        time.Time `json:"updated_at"`
}

const userTrafficCycleCarrySchema = `
CREATE TABLE IF NOT EXISTS user_traffic_cycle_carry (
    username TEXT PRIMARY KEY,
    weighted_uplink REAL NOT NULL DEFAULT 0,
    weighted_downlink REAL NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateUserTrafficCycleCarry() error {
	_, err := r.db.Exec(userTrafficCycleCarrySchema)
	return err
}

// --- UserRoutedOutboundAction ---

type UserRoutedOutboundAction struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}

const userRoutedOutboundActionsSchema = `
CREATE TABLE IF NOT EXISTS user_routed_outbound_actions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    action TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateUserRoutedOutboundActions() error {
	_, err := r.db.Exec(userRoutedOutboundActionsSchema)
	return err
}

// --- UserSubaccount ---

type UserSubaccount struct {
	ID              int64     `json:"id"`
	Username        string    `json:"username"`
	RoutedNodeID    int64     `json:"routed_node_id"`
	Email           string    `json:"email"`
	CredentialJSON  string    `json:"credential_json"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

const userSubaccountsSchema = `
CREATE TABLE IF NOT EXISTS user_subaccounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    routed_node_id INTEGER NOT NULL,
    email TEXT NOT NULL,
    credential_json TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(routed_node_id, username),
    UNIQUE(routed_node_id, email),
    FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
);
`

func (r *TrafficRepository) migrateUserSubaccounts() error {
	_, err := r.db.Exec(userSubaccountsSchema)
	return err
}

// --- UserAPIToken ---

type UserAPIToken struct {
	ID         int64      `json:"id"`
	Username   string     `json:"username"`
	Name       string     `json:"name"`
	TokenHash  string     `json:"-"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

const userAPITokensSchema = `
CREATE TABLE IF NOT EXISTS user_api_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    username     TEXT NOT NULL,
    name         TEXT NOT NULL DEFAULT '',
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP
);
`

func (r *TrafficRepository) migrateUserAPITokens() error {
	_, err := r.db.Exec(userAPITokensSchema)
	return err
}
