// Package cli 는 통합 nullus CLI 의 명령 표면이다 (트랙 A).
//
// 규율은 Automation 계약(docs/11_기능설계/Nullus_CLI_Automation_계약.md)을
// 따른다 — stdout 은 데이터, stderr 는 사람, exit code 는 pkg/nullusclient 의
// Kind 값과 일치한다. 플랫폼 로직은 없다: 서버 API 를 호출하고 결과를 보여줄
// 뿐이다.
package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/cloud-nullus/draft/pkg/nullusclient"
	"github.com/spf13/cobra"
)

// version 은 릴리스 파이프라인(R-1)이 ldflags 로 주입한다.
var version = "dev"

// Main 은 CLI 진입점이다. 프로세스 exit code 를 반환한다 —
// cmd/nullus 가 os.Exit 에 그대로 넘긴다.
func Main(args []string, stdout, stderr io.Writer) int {
	root := newRoot(stdout, stderr)
	root.SetArgs(args)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, "오류:", err)

		var apiErr *nullusclient.APIError
		if errors.As(err, &apiErr) {
			return apiErr.Kind.ExitCode()
		}
		var usageErr *usageError
		if errors.As(err, &usageErr) {
			return 2
		}
		return 1
	}
	return 0
}

// usageError 는 사용법 문제(잘못된 플래그·설정 부재 등)를 exit 2 로 이끈다.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

type rootOptions struct {
	server string
	output string // "" (표) | "json"
}

func newRoot(stdout, stderr io.Writer) *cobra.Command {
	opts := &rootOptions{}

	root := &cobra.Command{
		Use:           "nullus",
		Short:         "Nullus 플랫폼 CLI — 반복·자동화·헤드리스",
		SilenceUsage:  true,
		SilenceErrors: true, // 에러 출력·exit code 는 Main 이 담당한다
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&opts.server, "server", "", "API 서버 주소 (기본: NULLUS_SERVER env → ~/.nullus/config)")
	root.PersistentFlags().StringVarP(&opts.output, "output", "o", "", "출력 형식: json (기본은 사람용 표)")

	root.AddCommand(newVersionCmd(stdout))
	root.AddCommand(newStackCmd(opts, stdout))
	return root
}

func newVersionCmd(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "CLI 버전 출력",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(stdout, "nullus %s\n", version)
			return nil
		},
	}
}

// client 는 설정 우선순위(플래그 > env > 파일)를 적용해 API 클라이언트를 만든다.
func (o *rootOptions) client() (*nullusclient.Client, error) {
	cfg, err := nullusclient.Load(nullusclient.Config{Server: o.server})
	if err != nil {
		return nil, err
	}
	c, err := nullusclient.New(cfg)
	if err != nil {
		return nil, &usageError{msg: err.Error()}
	}
	return c, nil
}
