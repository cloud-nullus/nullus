package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrganization_ValidateSlug_Valid(t *testing.T) {
	cases := []string{
		"nullus-team",
		"my-org",
		"org123",
		"a",
		"abc-123-def",
	}
	for _, slug := range cases {
		o := &Organization{Slug: slug}
		assert.NoError(t, o.ValidateSlug(), "slug %q should be valid", slug)
	}
}

func TestOrganization_ValidateSlug_Invalid(t *testing.T) {
	cases := []string{
		"",
		"My-Org",
		"org_name",
		"-leading-hyphen",
		"trailing-hyphen-",
		"has space",
		"UPPER",
		"org--double",
	}
	for _, slug := range cases {
		o := &Organization{Slug: slug}
		assert.Error(t, o.ValidateSlug(), "slug %q should be invalid", slug)
	}
}
