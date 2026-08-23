// nullus 는 Nullus 플랫폼의 통합 CLI 다.
//
// 웹 UI 가 탐색·마법사(노코드)를 맡고, 이 CLI 는 반복·자동화·헤드리스를
// 맡는다 — 둘은 같은 /api/v1/* REST 를 쓴다. 설계는
// docs/11_기능설계/Nullus_CLI_컨셉.md, 사용법은
// docs/20_개발가이드/Nullus_CLI_사용_가이드.md.
package main

import (
	"os"

	"github.com/cloud-nullus/draft/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
