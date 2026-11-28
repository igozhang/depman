package model

import "time"

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RoleSuper = site igokey; RoleManager = scoped project key.
const (
	RoleSuper   = "super"
	RoleManager = "manager"
)

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type ProjectInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Environment struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"` // dev | uat | pro
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type EnvironmentInput struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// Credential types: git | harbor | ssh | api
type Credential struct {
	ID        string    `json:"id"`
	EnvID     string    `json:"envId"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Host      string    `json:"host"`
	Port      int       `json:"port,omitempty"`
	Username  string    `json:"username"`
	SecretSet bool      `json:"secretSet"`
	Secret    string    `json:"secret,omitempty"` // only when reveal=1 and authorized
	Extra     string    `json:"extra,omitempty"`  // JSON extras (repo path, etc.)
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CredentialInput struct {
	EnvID    string `json:"envId"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Secret   string `json:"secret"`
	Extra    string `json:"extra"`
}

type ProjectEnv struct {
	ID           string     `json:"id"`
	ProjectID    string     `json:"projectId"`
	EnvID        string     `json:"envId"`
	Name         string     `json:"name"` // 发布任务名；可与环境一对多
	SSHCredID    string     `json:"sshCredId,omitempty"`
	Script       string     `json:"script"`
	WorkDir      string     `json:"workDir,omitempty"`
	LastStatus   string     `json:"lastStatus,omitempty"` // idle | running | ok | warn | failed
	LastDeployAt *time.Time `json:"lastDeployAt,omitempty"`
	DeployCount  int        `json:"deployCount"` // 累计发布次数（含已清理的历史）
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type ProjectEnvInput struct {
	ID        string `json:"id,omitempty"` // 有则更新，无则新建（不再按 project+env 唯一）
	ProjectID string `json:"projectId"`
	EnvID     string `json:"envId"`
	Name      string `json:"name"`
	SSHCredID string `json:"sshCredId"`
	Script    string `json:"script"`
	WorkDir   string `json:"workDir"`
}

type ProjectCapsule struct {
	Project
	EnvCount       int        `json:"envCount"`
	LastStatus     string     `json:"lastStatus,omitempty"`
	LastDeployAt   *time.Time `json:"lastDeployAt,omitempty"`
	LastEnvName    string     `json:"lastEnvName,omitempty"`
}

type ScopedKey struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	KeyHint     string    `json:"keyHint"` // first 2 + **** + last 2, never full after create
	ProjectIDs  []string  `json:"projectIds"`
	EnvIDs      []string  `json:"envIds"`
	ExpiresAt   time.Time `json:"expiresAt"`
	CreatedAt   time.Time `json:"createdAt"`
	CreatedBy   string    `json:"createdBy"`
	PlainKey    string    `json:"plainKey,omitempty"` // only on create response
}

type ScopedKeyInput struct {
	Label      string   `json:"label"`
	ProjectIDs []string `json:"projectIds"`
	EnvIDs     []string `json:"envIds"`
	TTLHours   int      `json:"ttlHours"` // default 24, max 168
}

type DeployJob struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"projectId"`
	EnvID       string     `json:"envId"`
	ProjectEnvID string    `json:"projectEnvId"`
	Status      string     `json:"status"` // pending | running | ok | warn | failed
	Script      string     `json:"script"`
	LogTail     string     `json:"logTail,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	CreatedBy   string     `json:"createdBy"`
}

type DeployInput struct {
	ProjectEnvID string `json:"projectEnvId"`
}

type SessionInfo struct {
	Role       string   `json:"role"`
	ProjectIDs []string `json:"projectIds,omitempty"`
	EnvIDs     []string `json:"envIds,omitempty"`
	Label      string   `json:"label,omitempty"`
}
