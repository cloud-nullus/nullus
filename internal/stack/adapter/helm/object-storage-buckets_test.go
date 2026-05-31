package helm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildBucketBootstrapScript_IncludesBucketsAndAlias(t *testing.T) {
	script := buildBucketBootstrapScript("http://minio.local:9000", "ak", "sk", []string{"a", "b"})

	assert.Contains(t, script, `mc alias set target "http://minio.local:9000" "ak" "sk"`)
	assert.Contains(t, script, "mc mb --ignore-existing target/a")
	assert.Contains(t, script, "mc mb --ignore-existing target/b")
	assert.Contains(t, script, "mc ls target/a")
	assert.Contains(t, script, "mc ls target/b")
}

func TestEscapeJSONPathKey(t *testing.T) {
	out := escapeJSONPathKey("secret-key.name")
	assert.Equal(t, `secret\-key\.name`, out)
	assert.False(t, strings.Contains(out, " "))
}
