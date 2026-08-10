package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 핸들러가 라우트 등록 메서드를 정의해 놓고 main.go 에서 호출하지 않으면, 컴파일도
// 테스트도 전부 통과하는데 그 엔드포인트만 404 가 된다. 실제로 이 방식으로 두 개가
// 조용히 빠져 있었다 — 스택별 파이프라인 조회와 재배포 기록이다. 재배포 기록은
// 프론트 화면이 호출하는 API 였는데도 아무 신호가 없었다.
//
// 핸들러 단위 테스트는 Register*Routes 를 직접 호출하므로 이 누락을 잡지 못한다.
// 여기서는 main.go 를 파싱해 "정의된 등록기가 전부 호출되는가" 를 본다.
//
// 의도적으로 등록하지 않는 것은 아래 목록에 사유와 함께 적는다. 목록에 없는데
// 호출되지 않으면 실패한다.
var intentionallyUnregistered = map[string]string{
	"CompatibilityHandler.RegisterAdminRoutes": "" +
		"호환성 매트릭스 admin CRUD 는 WithManageCompatibility 옵션으로 켜는 선택 기능이다. " +
		"옵션 없이 등록하면 핸들러가 스스로 빈 등록으로 빠져나가므로 배선하지 않는다. " +
		"활성화하려면 ManageCompatibility 유스케이스를 만들어 옵션으로 주입해야 한다.",
}

func TestEveryRouteRegistrarIsWiredInMain(t *testing.T) {
	repoRoot := repoRootDir(t)

	defined := definedRouteRegistrars(t, filepath.Join(repoRoot, "internal"))
	if len(defined) == 0 {
		t.Fatal("라우트 등록기를 하나도 찾지 못했다 — 탐색 경로가 잘못됐을 수 있다")
	}
	called := calledRouteRegistrars(t, filepath.Join(repoRoot, "cmd", "api", "main.go"))

	for key := range defined {
		if _, ok := called[key]; ok {
			continue
		}
		if reason, ok := intentionallyUnregistered[key]; ok {
			t.Logf("의도적으로 등록하지 않음: %s — %s", key, reason)
			continue
		}
		t.Errorf("%s 가 정의돼 있으나 main.go 에서 호출되지 않는다. "+
			"배선을 추가하거나, 의도한 것이라면 intentionallyUnregistered 에 사유와 함께 등록하라", key)
	}

	// 목록이 낡는 것도 막는다 — 배선한 뒤 목록에 남겨 두면 다음 누락을 가린다.
	for key := range intentionallyUnregistered {
		if _, ok := defined[key]; !ok {
			t.Errorf("intentionallyUnregistered 의 %s 는 더 이상 존재하지 않는 등록기다. 항목을 지워라", key)
			continue
		}
		if _, ok := called[key]; ok {
			t.Errorf("%s 는 이제 main.go 에서 호출된다. intentionallyUnregistered 에서 지워라", key)
		}
	}
}

// repoRootDir 은 cmd/api 기준으로 저장소 루트를 찾는다.
func repoRootDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("작업 디렉터리를 읽지 못했다: %v", err)
	}
	root := filepath.Join(wd, "..", "..")
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("저장소 루트를 찾지 못했다 (%s): %v", root, err)
	}
	return root
}

// definedRouteRegistrars 는 핸들러 패키지에서 Register...Routes 메서드를 모은다.
// 키는 "TypeName.MethodName" 이다.
func definedRouteRegistrars(t *testing.T, internalDir string) map[string]struct{} {
	t.Helper()
	out := map[string]struct{}{}

	err := filepath.Walk(internalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) != "handler" {
			return nil
		}

		file, perr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if perr != nil {
			return perr
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
				continue
			}
			name := fn.Name.Name
			if !strings.HasPrefix(name, "Register") || !strings.HasSuffix(name, "Routes") {
				continue
			}
			if recv := receiverTypeName(fn.Recv.List[0].Type); recv != "" {
				out[recv+"."+name] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("핸들러 패키지를 훑지 못했다: %v", err)
	}
	return out
}

// calledRouteRegistrars 는 main.go 에서 호출된 등록기를 모은다.
//
// 변수 이름만으로는 어느 핸들러인지 알 수 없으므로, `x := pkg.NewFoo(...)` 형태의
// 생성자 호출로 변수→타입을 먼저 매핑한 뒤 메서드 호출을 대응시킨다.
func calledRouteRegistrars(t *testing.T, mainPath string) map[string]struct{} {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), mainPath, nil, 0)
	if err != nil {
		t.Fatalf("main.go 를 파싱하지 못했다: %v", err)
	}

	varType := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		if typeName := constructedTypeName(assign.Rhs[0]); typeName != "" {
			varType[ident.Name] = typeName
		}
		return true
	})

	out := map[string]struct{}{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		method := sel.Sel.Name
		if !strings.HasPrefix(method, "Register") || !strings.HasSuffix(method, "Routes") {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if typeName := varType[recv.Name]; typeName != "" {
			out[typeName+"."+method] = struct{}{}
		}
		return true
	})
	return out
}

func receiverTypeName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// constructedTypeName 은 `pkg.NewFoo(...)` 에서 "Foo" 를 뽑는다.
// `pkg.NewFoo(...).WithBar(...)` 처럼 체이닝된 경우도 안쪽 생성자를 찾는다.
func constructedTypeName(expr ast.Expr) string {
	for {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			return ""
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return ""
		}
		if strings.HasPrefix(sel.Sel.Name, "New") {
			return strings.TrimPrefix(sel.Sel.Name, "New")
		}
		// 체이닝이면 한 단계 안쪽으로 들어간다.
		expr = sel.X
	}
}
