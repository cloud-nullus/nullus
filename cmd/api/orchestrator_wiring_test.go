package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// Orchestrator 옵션을 정의해 놓고 main.go 에서 넘기지 않으면, 컴파일도 테스트도
// 전부 통과하는데 그 기능만 조용히 죽는다. 단위 테스트가 옵션을 직접 주입하기
// 때문에 초록불이 유지된다.
//
// 실제로 WithResourceDefaultRepository 가 그랬다. 이 옵션이 없으면
// loadResourceDefault 가 repo nil 을 보고 바로 빠져나가, 모든 차트가 resources
// 없이 설치된다. 클러스터에서 ArgoCD·kube-prometheus-stack 파드의 requests/limits
// 가 전부 비어 있었고(helm get values argo-cd 에 resources 가 한 줄도 없었다),
// 화면의 "0m / 0Mi" 는 그 결과를 정확히 보여주고 있었다.
//
// 의도적으로 넘기지 않는 옵션은 사유와 함께 아래에 적는다.
var intentionallyUnpassedOrchestratorOptions = map[string]string{
	"WithSharedClusterScopedComponents": "" +
		"클러스터 단위 공유 컴포넌트는 에어갭·멀티스택 배포에서만 켠다. " +
		"기본 배선에서 켜면 한 클러스터에 스택을 두 개 깔 때 소유권이 충돌한다.",
}

func TestOrchestratorOptionsArePassedInMain(t *testing.T) {
	repoRoot := repoRootDir(t)

	defined := orchestratorOptionsDefinedIn(t, filepath.Join(repoRoot, "internal", "stack", "adapter", "helm", "orchestrator.go"))
	if len(defined) == 0 {
		t.Fatal("Orchestrator 옵션을 하나도 찾지 못했다 — 테스트가 파일 구조를 잘못 읽고 있다")
	}

	passed := callNamesIn(t, filepath.Join(repoRoot, "cmd", "api", "main.go"))

	for _, name := range defined {
		if _, ok := intentionallyUnpassedOrchestratorOptions[name]; ok {
			continue
		}
		if !passed[name] {
			t.Errorf("%s 가 main.go 에서 Orchestrator 에 전달되지 않는다. "+
				"배선하거나, 의도적이라면 intentionallyUnpassedOrchestratorOptions 에 사유를 적어라", name)
		}
	}
}

// orchestrator.go 에서 OrchestratorOption 을 반환하는 최상위 함수 이름을 모은다.
func orchestratorOptionsDefinedIn(t *testing.T, path string) []string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("%s 파싱 실패: %v", path, err)
	}

	var names []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Type.Results == nil {
			continue
		}
		for _, result := range fn.Type.Results.List {
			if ident, ok := result.Type.(*ast.Ident); ok && ident.Name == "OrchestratorOption" {
				names = append(names, fn.Name.Name)
				break
			}
		}
	}
	return names
}

// main.go 안에서 호출되는 함수 이름(패키지 한정자 제외)을 모은다.
func callNamesIn(t *testing.T, path string) map[string]bool {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("%s 파싱 실패: %v", path, err)
	}

	called := map[string]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			called[fn.Name] = true
		case *ast.SelectorExpr:
			called[fn.Sel.Name] = true
		}
		return true
	})
	return called
}
