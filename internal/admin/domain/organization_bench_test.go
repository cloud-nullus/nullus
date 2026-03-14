package domain

import "testing"

func BenchmarkOrganization_ValidateSlug(b *testing.B) {
	org := &Organization{Slug: "my-valid-slug-123"}
	for i := 0; i < b.N; i++ {
		org.ValidateSlug()
	}
}
