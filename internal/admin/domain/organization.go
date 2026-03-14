package domain

import (
	"fmt"
	"regexp"
	"time"
)

// OrgStatus represents the status of an organization.
type OrgStatus string

const (
	OrgStatusActive   OrgStatus = "active"
	OrgStatusInactive OrgStatus = "inactive"
)

// Organization represents an organization (tenant) in the platform.
type Organization struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Domain         string    `json:"domain"`
	Status         OrgStatus `json:"status"`
	DefaultAdminID string    `json:"default_admin_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

var slugRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateSlug validates that the slug contains only lowercase letters, numbers, and hyphens.
func (o *Organization) ValidateSlug() error {
	if !slugRegex.MatchString(o.Slug) {
		return fmt.Errorf("invalid slug %q: must contain only lowercase letters, numbers, and hyphens", o.Slug)
	}
	return nil
}
