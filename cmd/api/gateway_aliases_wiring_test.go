package main

import (
	"os"
	"strings"
	"testing"
)

// 복구 뒤 게이트웨이 주소 재설정은 스택 모듈이 하고 백업 모듈이 부른다. main 에서 꽂지 않으면 컴파일도
// 테스트도 통과하는데 복구가 그 단계를 조용히 건너뛰어, 다른 클러스터로 되돌린 스택의 파이프라인이
// "Could not resolve host" 로 멈춘다.
func TestGatewayAliasRepinIsWiredIntoBackup(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.Join(strings.Fields(string(raw)), " ")
	for _, want := range []string{
		"stackuc.NewGatewayAliasRepin(",
		"stackhelm.NewGatewayAddressReader(",
		"GatewayAliases: gatewayAliasRepinUC,",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("main.go 에 %q 가 없다", want)
		}
	}
}
