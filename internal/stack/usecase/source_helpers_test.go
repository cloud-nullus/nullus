package usecase

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// readSourceFile 는 같은 패키지의 소스를 읽는다. 호출 **순서**처럼 실행 없이
// 확인해야 하는 규칙을 고정할 때 쓴다.
func readSourceFile(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	require.NoError(t, err)
	return string(raw)
}

// indexOfCall 은 함수 호출이 나타나는 위치를 돌려준다. 없으면 -1 이다.
func indexOfCall(src, fn string) int {
	return strings.Index(src, "uc."+fn+"(")
}
