package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newStackCmd(opts *rootOptions, stdout io.Writer) *cobra.Command {
	stack := &cobra.Command{
		Use:   "stack",
		Short: "스택 조회·배포",
	}
	stack.AddCommand(newStackLsCmd(opts, stdout))
	return stack
}

// stackListItem 은 CLI 가 소유하는 출력 스키마다 (Automation 계약 §3 —
// 서버 응답을 그대로 통과시키지 않는다). 서버가 필드를 더 보내도 여기 있는
// 것만 내보낸다.
type stackListItem struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	State       string    `json:"state"`
	ClusterName string    `json:"cluster_name"`
	Namespace   string    `json:"namespace"`
	CreatedAt   time.Time `json:"created_at"`
}

func newStackLsCmd(opts *rootOptions, stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "스택 목록",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := opts.client()
			if err != nil {
				return err
			}

			var resp struct {
				Items []stackListItem `json:"items"`
				Total int             `json:"total"`
			}
			if err := c.Do(cmd.Context(), http.MethodGet, "/api/v1/stacks", nil, &resp); err != nil {
				return err
			}

			if opts.output == "json" {
				enc := json.NewEncoder(stdout)
				return enc.Encode(map[string]any{"items": resp.Items})
			}

			w := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tSTATE\tCLUSTER\tNAMESPACE\tCREATED")
			for _, it := range resp.Items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					it.ID, it.Name, it.State, orDash(it.ClusterName), it.Namespace,
					it.CreatedAt.Format("2006-01-02 15:04"))
			}
			return w.Flush()
		},
	}
}

// orDash 는 모르는 값을 모른다고 표시한다 — 빈 칸은 누락처럼 읽힌다.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
