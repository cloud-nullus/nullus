package domain

import (
	"errors"
	"strings"
	"testing"
)

const egovWebSampleCommit = "158daebd2bfdab951e90ad526c3d2a6b6b42513b"

func validSource() *TemplateSource {
	return &TemplateSource{
		RepoURL:      "https://github.com/eGovFramework/egovframe-web-sample",
		Commit:       egovWebSampleCommit,
		Label:        "v5.0.0",
		BuildProfile: BuildProfileMavenWAR,
	}
}

func TestTemplateSource_Validate_Success(t *testing.T) {
	if err := validSource().Validate(); err != nil {
		t.Fatalf("expected valid source, got %v", err)
	}
}

func TestTemplateSource_Validate_EmptyLabelAndProfile(t *testing.T) {
	s := validSource()
	s.Label = ""
	s.BuildProfile = BuildProfileRepository
	if err := s.Validate(); err != nil {
		t.Fatalf("label and profile are optional, got %v", err)
	}
}

func TestTemplateSource_Validate_Invalid(t *testing.T) {
	cases := map[string]func(*TemplateSource){
		"empty repo url":    func(s *TemplateSource) { s.RepoURL = "" },
		"http repo url":     func(s *TemplateSource) { s.RepoURL = "http://github.com/a/b" },
		"no host":           func(s *TemplateSource) { s.RepoURL = "https:///a/b" },
		"no repo path":      func(s *TemplateSource) { s.RepoURL = "https://github.com" },
		"repo url too long": func(s *TemplateSource) { s.RepoURL = "https://github.com/" + strings.Repeat("a", 500) },
		// 태그·브랜치는 받지 않는다 — 태그는 다시 찍힐 수 있다(eGov 웹 샘플이 실제로 그랬다).
		"tag instead of commit": func(s *TemplateSource) { s.Commit = "v5.0.0" },
		"short commit":          func(s *TemplateSource) { s.Commit = "158daebd2bfd" },
		"uppercase commit":      func(s *TemplateSource) { s.Commit = strings.ToUpper(egovWebSampleCommit) },
		"empty commit":          func(s *TemplateSource) { s.Commit = "" },
		"label too long":        func(s *TemplateSource) { s.Label = strings.Repeat("v", 101) },
		"unknown profile":       func(s *TemplateSource) { s.BuildProfile = "gradle" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := validSource()
			mutate(s)
			err := s.Validate()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !errors.Is(err, ErrTemplateInvalid) {
				t.Fatalf("expected ErrTemplateInvalid, got %v", err)
			}
		})
	}
}

func TestTemplateSource_ShortCommit(t *testing.T) {
	if got := validSource().ShortCommit(); got != "158daebd2bfd" {
		t.Fatalf("ShortCommit() = %q, want 12 chars", got)
	}
}

func TestPipelineTemplate_Validate_ChecksSource(t *testing.T) {
	tmpl := validTemplate()
	tmpl.Source = validSource()
	if err := tmpl.Validate(); err != nil {
		t.Fatalf("expected valid template with source, got %v", err)
	}

	tmpl.Source.Commit = "main"
	if err := tmpl.Validate(); !errors.Is(err, ErrTemplateInvalid) {
		t.Fatalf("expected ErrTemplateInvalid for a branch name, got %v", err)
	}
}

func TestBuildProfile_IsValid(t *testing.T) {
	for _, p := range []BuildProfile{BuildProfileRepository, BuildProfileSpringBoot, BuildProfileMavenWAR} {
		if !p.IsValid() {
			t.Errorf("%q should be valid", p)
		}
	}
	for _, p := range []BuildProfile{"gradle", "SPRING-BOOT", " maven-war"} {
		if p.IsValid() {
			t.Errorf("%q should be invalid", p)
		}
	}
}

func TestBuildProfile_Dockerfile(t *testing.T) {
	cases := map[BuildProfile]struct{ replaces, generates bool }{
		// 원본 Dockerfile 이 우리 CI(docker build .)로 돌지 않는 경우 — 원본이 미리 빌드한 WAR 를 복사한다.
		BuildProfileMavenWAR: {replaces: true, generates: true},
		// 원본 Dockerfile 이 있으면 그것을 쓰고, 없으면 만든다 — eGov 부트 샘플 v5.0.0 에는 Dockerfile 이 없다.
		BuildProfileSpringBoot: {replaces: false, generates: true},
		BuildProfileRepository: {replaces: false, generates: false},
	}
	for p, want := range cases {
		assert := func(name string, got, want bool) {
			if got != want {
				t.Errorf("%q.%s() = %v, want %v", p, name, got, want)
			}
		}
		assert("ReplacesDockerfile", p.ReplacesDockerfile(), want.replaces)
		assert("GeneratesDockerfile", p.GeneratesDockerfile(), want.generates)
	}
}
