package port

import "testing"

func TestCommitFile_IsBinary(t *testing.T) {
	cases := map[string]struct {
		content string
		want    bool
	}{
		"plain text":       {"FROM nginx:alpine\n", false},
		"korean text":      {"# 전자정부 표준프레임워크\n", false},
		"empty":            {"", false},
		"invalid utf-8":    {"\xff\xd8\xff\xe0JFIF", true},
		"nul byte":         {"wOFF\x00\x01\x00\x00", true},
		"utf-8 with a nul": {"abc\x00def", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := (CommitFile{Path: "f", Content: tc.content}).IsBinary(); got != tc.want {
				t.Fatalf("IsBinary() = %v, want %v", got, tc.want)
			}
		})
	}
}
