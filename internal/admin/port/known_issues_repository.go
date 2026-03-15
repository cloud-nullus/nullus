package port

import "context"

type KnownIssue struct {
	ID          string
	Severity    string
	Title       string
	Description string
	Workaround  string
	Status      string
}

type KnownIssuesRepository interface {
	List(ctx context.Context) ([]KnownIssue, error)
}
