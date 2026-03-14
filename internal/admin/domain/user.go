package domain

import "time"

// Role represents a user role in the system.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleDevOps    Role = "devops"
	RoleDeveloper Role = "developer"
)

// User represents a platform user.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	OrgID     string    `json:"org_id"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CanAccess checks if the user has one of the required roles.
func (u *User) CanAccess(requiredRoles ...Role) bool {
	for _, r := range requiredRoles {
		if u.Role == r {
			return true
		}
	}
	return false
}
