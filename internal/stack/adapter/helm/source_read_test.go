package helm

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// readSourceFile 은 같은 패키지의 소스를 읽는다.
// 구조적 계약(어떤 헬퍼를 쓰는가)을 검사할 때 쓴다.
func readSourceFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	require.NoErrorf(t, err, "%s 를 읽지 못했다", name)
	return string(b)
}

// functionBody 는 시그니처로 시작하는 함수의 본문을 잘라 낸다.
func functionBody(t *testing.T, src, signature string) string {
	t.Helper()
	start := strings.Index(src, signature)
	require.GreaterOrEqualf(t, start, 0, "%s 를 찾지 못했다", signature)
	rest := src[start:]
	if end := strings.Index(rest[1:], "\nfunc "); end >= 0 {
		return rest[:end]
	}
	return rest
}
