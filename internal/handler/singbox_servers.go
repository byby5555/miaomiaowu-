package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"miaomiaowu/internal/auth"
	"miaomiaowu/internal/logger"
	"miaomiaowu/internal/singboxops"
	"miaomiaowu/internal/storage"
)

// singboxServersHandler manages remote sing-box servers.
type singboxServersHandler struct {
	repo *storage.TrafficRepository
}

// NewSingboxServersHandler creates the sing-box servers admin handler.
func NewSingboxServersHandler(repo *storage.TrafficRepository) http.Handler {
	return &singboxServersHandler{repo: repo}
}

// singboxServerRequest is the create/update payload from the frontend.
type singboxServerRequest struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	SSHUser     string `json:"ssh_user"`
	AuthType    string `json:"auth_type"`
	AuthData    string `json:"auth_data"` // password or private key (empty on update = unchanged)
	ConfigPath  string `json:"config_path"`
	APIPort     int    `json:"api_port"`
	SingboxPort int    `json:"singbox_port"`
	SingboxUser string `json:"singbox_user"`
	Enabled     *bool  `json:"enabled"`
}

// singboxServerResponse is what we return to the frontend (never includes auth data).
type singboxServerResponse struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	SSHUser       string `json:"ssh_user"`
	AuthType      string `json:"auth_type"`
	HasAuth       bool   `json:"has_auth"`
	ConfigPath    string `json:"config_path"`
	APIPort       int    `json:"api_port"`
	SingboxPort   int    `json:"singbox_port"`
	SingboxUser   string `json:"singbox_user"`
	Enabled       bool   `json:"enabled"`
	HasStatus     string `json:"has_status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func toSingboxServerResponse(s storage.SingboxServer, hasAuth bool) singboxServerResponse {
	return singboxServerResponse{
		ID:          s.ID,
		Name:        s.Name,
		Host:        s.Host,
		Port:        s.Port,
		SSHUser:     s.SSHUser,
		AuthType:    s.AuthType,
		HasAuth:     hasAuth,
		ConfigPath:  s.ConfigPath,
		APIPort:     s.APIPort,
		SingboxPort: s.SingboxPort,
		SingboxUser: s.SingboxUser,
		Enabled:     s.Enabled,
		HasStatus:   s.HasStatus,
		CreatedAt:   s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   s.UpdatedAt.Format(time.RFC3339),
	}
}

func (h *singboxServersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.repo == nil {
		writeError(w, http.StatusInternalServerError, errors.New("repository not configured"))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/admin/singbox-servers")
	path = strings.Trim(path, "/")
	segments := strings.Split(path, "/")

	switch {
	case len(segments) == 1 && segments[0] == "":
		// collection
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreate(w, r)
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
	case len(segments) >= 1 && segments[0] != "":
		id, err := strconv.ParseInt(segments[0], 10, 64)
		if err != nil {
			writeBadRequest(w, "无效的服务器 ID")
			return
		}
		action := ""
		if len(segments) >= 2 {
			action = segments[1]
		}
		h.handleItem(w, r, id, action)
	default:
		http.NotFound(w, r)
	}
}

func (h *singboxServersHandler) handleList(w http.ResponseWriter, r *http.Request) {
	servers, err := h.repo.ListSingboxServers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	resp := make([]singboxServerResponse, 0, len(servers))
	for _, s := range servers {
		auth, err := h.repo.GetSingboxServerAuth(r.Context(), s.ID)
		resp = append(resp, toSingboxServerResponse(s, err == nil && auth.AuthData != ""))
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *singboxServersHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req singboxServerRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeBadRequest(w, "无效的请求体: "+err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	srv := storage.SingboxServer{
		Name:        req.Name,
		Host:        req.Host,
		Port:        req.Port,
		SSHUser:     req.SSHUser,
		ConfigPath:  req.ConfigPath,
		APIPort:     req.APIPort,
		SingboxPort: req.SingboxPort,
		SingboxUser: req.SingboxUser,
		Enabled:     enabled,
	}
	auth := storage.SingboxServerAuth{
		AuthType: req.AuthType,
		AuthData: req.AuthData,
	}
	created, err := h.repo.CreateSingboxServer(r.Context(), srv, auth)
	if err != nil {
		if errors.Is(err, storage.ErrSingboxServerExists) {
			writeBadRequest(w, "服务器名称已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusCreated, toSingboxServerResponse(created, auth.AuthData != ""))
}

func (h *singboxServersHandler) handleItem(w http.ResponseWriter, r *http.Request, id int64, action string) {
	switch {
	case action == "" && r.Method == http.MethodPut:
		h.handleUpdate(w, r, id)
	case action == "" && r.Method == http.MethodDelete:
		h.handleDelete(w, r, id)
	case action == "test" && r.Method == http.MethodPost:
		h.handleTest(w, r, id)
	case action == "status" && r.Method == http.MethodPost:
		h.handleStatus(w, r, id)
	case action == "deploy" && r.Method == http.MethodPost:
		h.handleDeploy(w, r, id)
	case action == "restart" && r.Method == http.MethodPost:
		h.handleRestart(w, r, id)
	case action == "sync-node" && r.Method == http.MethodPost:
		h.handleSyncNode(w, r, id)
	default:
		methodNotAllowed(w, http.MethodPut, http.MethodDelete, http.MethodPost)
	}
}

func (h *singboxServersHandler) handleUpdate(w http.ResponseWriter, r *http.Request, id int64) {
	var req singboxServerRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeBadRequest(w, "无效的请求体: "+err.Error())
		return
	}
	existing, err := h.repo.GetSingboxServer(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrSingboxServerNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	// 保留未提交字段
	if req.Name == "" {
		req.Name = existing.Name
	}
	if req.Host == "" {
		req.Host = existing.Host
	}
	if req.Port <= 0 {
		req.Port = existing.Port
	}
	if req.SSHUser == "" {
		req.SSHUser = existing.SSHUser
	}
	if req.ConfigPath == "" {
		req.ConfigPath = existing.ConfigPath
	}
	if req.APIPort <= 0 {
		req.APIPort = existing.APIPort
	}
	if req.SingboxPort <= 0 {
		req.SingboxPort = existing.SingboxPort
	}
	enabled := existing.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	srv := storage.SingboxServer{
		ID:          id,
		Name:        req.Name,
		Host:        req.Host,
		Port:        req.Port,
		SSHUser:     req.SSHUser,
		ConfigPath:  req.ConfigPath,
		APIPort:     req.APIPort,
		SingboxPort: req.SingboxPort,
		SingboxUser: req.SingboxUser,
		Enabled:     enabled,
	}
	// 只有提交了 auth_data 才更新认证信息
	var auth *storage.SingboxServerAuth
	if req.AuthData != "" {
		auth = &storage.SingboxServerAuth{AuthType: req.AuthType, AuthData: req.AuthData}
	}
	updated, err := h.repo.UpdateSingboxServer(r.Context(), srv, auth)
	if err != nil {
		if errors.Is(err, storage.ErrSingboxServerExists) {
			writeBadRequest(w, "服务器名称已存在")
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	hasAuth := auth != nil
	respondJSON(w, http.StatusOK, toSingboxServerResponse(updated, hasAuth))
}

func (h *singboxServersHandler) handleDelete(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.repo.DeleteSingboxServer(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrSingboxServerNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// buildOpsClient assembles a singboxops.Client from stored server + auth data.
func (h *singboxServersHandler) buildOps(ctx context.Context, id int64) (*singboxops.Client, *storage.SingboxServer, error) {
	srv, err := h.repo.GetSingboxServer(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	auth, err := h.repo.GetSingboxServerAuth(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	ops, err := singboxops.New(singboxops.Remote{
		Host:        srv.Host,
		Port:        srv.Port,
		SSHUser:     srv.SSHUser,
		AuthType:    auth.AuthType,
		AuthData:    auth.AuthData,
		ConfigPath:  srv.ConfigPath,
		APIPort:     srv.APIPort,
		SingboxPort: srv.SingboxPort,
		SingboxUser: srv.SingboxUser,
	})
	if err != nil {
		return nil, nil, err
	}
	return ops, &srv, nil
}

func (h *singboxServersHandler) handleTest(w http.ResponseWriter, r *http.Request, id int64) {
	ops, _, err := h.buildOps(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := ops.TestConnection(r.Context()); err != nil {
		h.repo.SetSingboxServerStatus(r.Context(), id, "连接失败: "+err.Error())
		writeError(w, http.StatusBadGateway, err)
		return
	}
	_ = h.repo.SetSingboxServerStatus(r.Context(), id, "连接正常")
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *singboxServersHandler) handleStatus(w http.ResponseWriter, r *http.Request, id int64) {
	ops, _, err := h.buildOps(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	st, err := ops.GetStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.repo.SetSingboxServerStatus(r.Context(), id, st.Message)
	respondJSON(w, http.StatusOK, st)
}

func (h *singboxServersHandler) handleDeploy(w http.ResponseWriter, r *http.Request, id int64) {
	ops, srv, err := h.buildOps(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	content, err := ops.GenerateConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := ops.Deploy(r.Context(), content); err != nil {
		_ = h.repo.SetSingboxServerStatus(r.Context(), id, "部署失败: "+err.Error())
		writeError(w, http.StatusBadGateway, err)
		return
	}
	_ = h.repo.SetSingboxServerStatus(r.Context(), id, "已部署 "+time.Now().Format("15:04:05"))
	logger.Info("[SingboxServer] 部署配置", "server_id", id, "server", srv.Name)
	respondJSON(w, http.StatusOK, map[string]bool{"deployed": true})
}

func (h *singboxServersHandler) handleRestart(w http.ResponseWriter, r *http.Request, id int64) {
	ops, srv, err := h.buildOps(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := ops.Restart(r.Context()); err != nil {
		_ = h.repo.SetSingboxServerStatus(r.Context(), id, "重启失败: "+err.Error())
		writeError(w, http.StatusBadGateway, err)
		return
	}
	_ = h.repo.SetSingboxServerStatus(r.Context(), id, "已重启 "+time.Now().Format("15:04:05"))
	logger.Info("[SingboxServer] 重启服务", "server_id", id, "server", srv.Name)
	respondJSON(w, http.StatusOK, map[string]bool{"restarted": true})
}

// handleSyncNode creates or updates a node in the nodes table for the sing-box server,
// making it available for subscription output.
func (h *singboxServersHandler) handleSyncNode(w http.ResponseWriter, r *http.Request, id int64) {
	ops, srv, err := h.buildOps(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Generate Clash-compatible proxy JSON for the server's mixed inbound
	clashConfig, err := ops.GenerateClashProxy()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// Use the authenticated admin's username as the node owner
	username := auth.UsernameFromContext(r.Context())
	if username == "" {
		writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}

	node, err := h.repo.SyncSingboxServerNode(r.Context(), id, username, clashConfig)
	if err != nil {
		logger.Error("[SingboxServer] 同步节点失败", "server_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	_ = h.repo.SetSingboxServerStatus(r.Context(), id, "节点已同步")
	logger.Info("[SingboxServer] 同步节点", "server_id", id, "server", srv.Name, "node_id", node.ID)
	respondJSON(w, http.StatusOK, map[string]any{
		"synced":   true,
		"node_id":  node.ID,
		"node_name": node.NodeName,
	})
}