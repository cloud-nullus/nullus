package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_CanAccess_WithMatchingRole(t *testing.T) {
	u := &User{ID: "u1", Role: RoleAdmin}

	assert.True(t, u.CanAccess(RoleAdmin))
	assert.True(t, u.CanAccess(RoleAdmin, RoleDevOps))
}

func TestUser_CanAccess_WithNonMatchingRole(t *testing.T) {
	u := &User{ID: "u1", Role: RoleDeveloper}

	assert.False(t, u.CanAccess(RoleAdmin))
	assert.False(t, u.CanAccess(RoleAdmin, RoleDevOps))
}

func TestUser_CanAccess_EmptyRoles(t *testing.T) {
	u := &User{ID: "u1", Role: RoleAdmin}

	assert.False(t, u.CanAccess())
}

func TestRole_Constants(t *testing.T) {
	assert.Equal(t, Role("admin"), RoleAdmin)
	assert.Equal(t, Role("devops"), RoleDevOps)
	assert.Equal(t, Role("developer"), RoleDeveloper)
}
