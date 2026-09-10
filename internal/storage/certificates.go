package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// --- Certificate ---

type Certificate struct {
	ID             int64      `json:"id"`
	Domain         string     `json:"domain"`
	Email          string     `json:"email"`
	Provider       string     `json:"provider"` // letsencrypt | zerossl | buypass
	CertPath       string     `json:"cert_path"`
	KeyPath        string     `json:"key_path"`
	CertPEM        string     `json:"-"`
	KeyPEM         string     `json:"-"`
	Status         string     `json:"status"` // pending | valid | expired | failed
	ExpiryDate     *time.Time `json:"expiry_date"`
	IssueDate      *time.Time `json:"issue_date"`
	AutoRenew      bool       `json:"auto_renew"`
	ChallengeMode  string     `json:"challenge_mode"` // standalone | webroot | dns | manual
	WebrootPath    string     `json:"webroot_path"`
	RemoteServerID int64      `json:"remote_server_id"`
	Message        string     `json:"message"`
	DNSProviderID  int64      `json:"dns_provider_id"`
	DeployTarget   string     `json:"deploy_target"` // none | remote_server | local
	DeployCertPath string     `json:"deploy_cert_path"`
	DeployKeyPath  string     `json:"deploy_key_path"`
	AutoDeploy     bool       `json:"auto_deploy"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

const certificatesSchema = `
CREATE TABLE IF NOT EXISTS certificates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL,
    email TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT 'letsencrypt',
    cert_path TEXT,
    key_path TEXT,
    cert_pem TEXT,
    key_pem TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'valid', 'expired', 'failed')),
    expiry_date TIMESTAMP,
    issue_date TIMESTAMP,
    auto_renew INTEGER NOT NULL DEFAULT 1,
    challenge_mode TEXT NOT NULL DEFAULT 'standalone' CHECK (challenge_mode IN ('standalone', 'webroot', 'dns', 'manual')),
    webroot_path TEXT,
    remote_server_id INTEGER NOT NULL DEFAULT 0,
    message TEXT,
    dns_provider_id INTEGER NOT NULL DEFAULT 0,
    deploy_target TEXT NOT NULL DEFAULT 'none',
    deploy_cert_path TEXT,
    deploy_key_path TEXT,
    auto_deploy INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(domain, remote_server_id)
);
`

func (r *TrafficRepository) migrateCertificates() error {
	_, err := r.db.Exec(certificatesSchema)
	return err
}

var ErrCertificateNotFound = errors.New("certificate not found")

func (r *TrafficRepository) CreateCertificate(ctx context.Context, c *Certificate) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO certificates (domain, email, provider, status, auto_renew, challenge_mode, remote_server_id, dns_provider_id, deploy_target, auto_deploy) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Domain, c.Email, c.Provider, c.Status, boolToInt(c.AutoRenew), c.ChallengeMode, c.RemoteServerID, c.DNSProviderID, c.DeployTarget, boolToInt(c.AutoDeploy))
	if err != nil {
		return 0, fmt.Errorf("create certificate: %w", err)
	}
	return res.LastInsertId()
}

func (r *TrafficRepository) ListCertificates(ctx context.Context) ([]Certificate, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, domain, email, provider, COALESCE(cert_path,''), COALESCE(key_path,''), status, expiry_date, issue_date, auto_renew, challenge_mode, COALESCE(webroot_path,''), remote_server_id, COALESCE(message,''), dns_provider_id, deploy_target, COALESCE(deploy_cert_path,''), COALESCE(deploy_key_path,''), auto_deploy, created_at, updated_at FROM certificates ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Certificate
	for rows.Next() {
		var c Certificate
		var autoRenew, autoDeploy int
		if err := rows.Scan(&c.ID, &c.Domain, &c.Email, &c.Provider, &c.CertPath, &c.KeyPath, &c.Status, &c.ExpiryDate, &c.IssueDate, &autoRenew, &c.ChallengeMode, &c.WebrootPath, &c.RemoteServerID, &c.Message, &c.DNSProviderID, &c.DeployTarget, &c.DeployCertPath, &c.DeployKeyPath, &autoDeploy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.AutoRenew = autoRenew != 0
		c.AutoDeploy = autoDeploy != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) GetCertificate(ctx context.Context, id int64) (Certificate, error) {
	var c Certificate
	var autoRenew, autoDeploy int
	err := r.db.QueryRowContext(ctx, `SELECT id, domain, email, provider, COALESCE(cert_path,''), COALESCE(key_path,''), status, expiry_date, issue_date, auto_renew, challenge_mode, COALESCE(webroot_path,''), remote_server_id, COALESCE(message,''), dns_provider_id, deploy_target, COALESCE(deploy_cert_path,''), COALESCE(deploy_key_path,''), auto_deploy, created_at, updated_at FROM certificates WHERE id = ?`, id).
		Scan(&c.ID, &c.Domain, &c.Email, &c.Provider, &c.CertPath, &c.KeyPath, &c.Status, &c.ExpiryDate, &c.IssueDate, &autoRenew, &c.ChallengeMode, &c.WebrootPath, &c.RemoteServerID, &c.Message, &c.DNSProviderID, &c.DeployTarget, &c.DeployCertPath, &c.DeployKeyPath, &autoDeploy, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return c, ErrCertificateNotFound
	}
	c.AutoRenew = autoRenew != 0
	c.AutoDeploy = autoDeploy != 0
	return c, err
}

func (r *TrafficRepository) UpdateCertificateStatus(ctx context.Context, id int64, status, message string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE certificates SET status = ?, message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, message, id)
	return err
}

func (r *TrafficRepository) DeleteCertificate(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM certificates WHERE id = ?`, id)
	return err
}

// --- DNSProvider ---

type DNSProvider struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	ProviderType string    `json:"provider_type"` // cloudflare | alidns | dnspod | ...
	Credentials  string    `json:"-"` // encrypted JSON, never expose
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

const dnsProvidersSchema = `
CREATE TABLE IF NOT EXISTS dns_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    provider_type TEXT NOT NULL,
    credentials TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

func (r *TrafficRepository) migrateDNSProviders() error {
	_, err := r.db.Exec(dnsProvidersSchema)
	return err
}

var ErrDNSProviderNotFound = errors.New("dns provider not found")

func (r *TrafficRepository) CreateDNSProvider(ctx context.Context, p *DNSProvider) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO dns_providers (name, provider_type, credentials) VALUES (?, ?, ?)`, p.Name, p.ProviderType, p.Credentials)
	if err != nil {
		return 0, fmt.Errorf("create dns provider: %w", err)
	}
	return res.LastInsertId()
}

func (r *TrafficRepository) ListDNSProviders(ctx context.Context) ([]DNSProvider, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, provider_type, created_at, updated_at FROM dns_providers ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DNSProvider
	for rows.Next() {
		var p DNSProvider
		if err := rows.Scan(&p.ID, &p.Name, &p.ProviderType, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *TrafficRepository) DeleteDNSProvider(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM dns_providers WHERE id = ?`, id)
	return err
}
