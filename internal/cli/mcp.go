package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cloud-nullus/draft/internal/mcp"
	"github.com/cloud-nullus/draft/pkg/nullusclient"
)

// EnvAllowWrite 는 --allow-write 의 env 대응이다 — MCP 호스트 설정(.mcp.json)
// 에서는 플래그 편집보다 env 주입이 쉬운 경우가 많다.
const EnvAllowWrite = "NULLUS_MCP_ALLOW_WRITE"

func newMCPCmd(opts *rootOptions, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP 서버 — AI 어시스턴트용 Nullus tool 표면",
	}
	cmd.AddCommand(newMCPServeCmd(opts, stderr))
	return cmd
}

func newMCPServeCmd(opts *rootOptions, stderr io.Writer) *cobra.Command {
	var allowWrite bool

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "MCP 서버를 stdio 로 기동",
		Long: `MCP 서버를 stdio transport 로 기동한다. stdout 은 프로토콜 전용이며
사람용 안내·경고는 전부 stderr 로 나간다 (Automation 계약 §2).

기본 표면은 읽기 tool 6종이다. 변경 tool 3종(stack_deploy·stack_rollback·
pipeline_deploy)은 --allow-write 또는 ` + EnvAllowWrite + `=true 옵트인 시에만
표면에 등록된다 — 미허용이면 list_tools 에 아예 나타나지 않는다.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := nullusclient.Load(nullusclient.Config{Server: opts.server})
			if err != nil {
				return err
			}
			c, err := nullusclient.New(cfg)
			if err != nil {
				return &usageError{msg: err.Error()}
			}
			// 토큰 부재는 기동 전에 로컬에서 판정한다 — 서버 401 을 기다리면
			// 모든 tool 호출이 실패하는 서버만 하나 떠 있게 된다.
			if cfg.Token == "" {
				return &authError{msg: fmt.Sprintf(
					"로그인 토큰이 없다 — `nullus login` 으로 로그인하거나 %s env 를 설정하라. 무인 환경은 `nullus auth bootstrap issue` 토큰을 쓴다",
					nullusclient.EnvToken)}
			}

			return mcp.Run(cmd.Context(), c, mcp.RunOptions{
				AllowWrite: resolveAllowWrite(cmd.Flags().Changed("allow-write"), allowWrite),
				Version:    version,
				ServerURL:  cfg.Server,
				Stderr:     stderr,
			})
		},
	}
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false,
		"변경 tool 3종 노출 ("+EnvAllowWrite+"=true 와 동일)")
	return cmd
}

// resolveAllowWrite 는 옵트인을 해석한다 — 플래그가 명시되면 그 값이 env 를
// 이긴다 (명시 --allow-write=false 로 env 를 끌 수 있어야 한다).
func resolveAllowWrite(flagSet, flagValue bool) bool {
	if flagSet {
		return flagValue
	}
	v, err := strconv.ParseBool(os.Getenv(EnvAllowWrite))
	return err == nil && v
}
