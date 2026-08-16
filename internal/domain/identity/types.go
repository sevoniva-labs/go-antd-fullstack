package identity

import "time"

type Organization struct {
	ID        string    `json:"id"`
	Key       string    `json:"org_key"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Permission struct {
	ID        string    `json:"id"`
	Key       string    `json:"permission_key"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Role struct {
	ID          string       `json:"id"`
	Key         string       `json:"role_key"`
	Name        string       `json:"name"`
	Permissions []Permission `json:"permissions"`
	CreatedAt   time.Time    `json:"created_at"`
}

type Session struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	LoginName   string    `json:"login_name"`
	DisplayName string    `json:"display_name"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	ClientIP    string    `json:"client_ip"`
	UserAgent   string    `json:"user_agent"`
	Current     bool      `json:"current"`
}

type User struct {
	ID                 string     `json:"id"`
	OrganizationID     string     `json:"organization_id"`
	LoginName          string     `json:"login_name"`
	DisplayName        string     `json:"display_name"`
	Status             string     `json:"status"`
	MustChangePassword bool       `json:"must_change_password"`
	FailedLoginCount   int        `json:"-"`
	LockedUntil        *time.Time `json:"locked_until,omitempty"`
	PasswordChangedAt  time.Time  `json:"-"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	Roles              []string   `json:"roles"`
	Permissions        []string   `json:"permissions,omitempty"`
}

func (u User) HasRole(keys ...string) bool {
	for _, have := range u.Roles {
		for _, want := range keys {
			if have == want {
				return true
			}
		}
	}
	return false
}

type APIToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Principal struct {
	Type               string    `json:"principal_type"`
	UserID             string    `json:"user_id"`
	OrganizationID     string    `json:"organization_id"`
	LoginName          string    `json:"login_name"`
	DisplayName        string    `json:"display_name"`
	Roles              []string  `json:"roles"`
	Permissions        []string  `json:"permissions,omitempty"`
	Scopes             []string  `json:"scopes,omitempty"`
	MustChangePassword bool      `json:"must_change_password"`
	SessionID          string    `json:"-"`
	PasswordChangedAt  time.Time `json:"-"`
}

func (p Principal) HasRole(keys ...string) bool {
	for _, have := range p.Roles {
		for _, want := range keys {
			if have == want {
				return true
			}
		}
	}
	return false
}
func (p Principal) HasPermission(keys ...string) bool {
	if p.Type == "TOKEN" {
		for _, want := range keys {
			allowed := false
			for _, scope := range p.Scopes {
				if scope == want || scope == "*" {
					allowed = true
					break
				}
			}
			if !allowed {
				return false
			}
		}
	}
	if p.HasRole("system_admin") {
		return true
	}
	for _, have := range p.Permissions {
		for _, want := range keys {
			if have == want {
				return true
			}
		}
	}
	return false
}
