package domain

import (
	"errors"
	"strings"
	"testing"
)

func validTemplate() *PipelineTemplate {
	return &PipelineTemplate{
		ID:      "team-backend-v1",
		Name:    "Team Backend",
		AppType: AppTypeBackend,
		Stages:  []string{"Build", "Deploy"},
	}
}

func TestPipelineTemplate_Validate_Success(t *testing.T) {
	if err := validTemplate().Validate(); err != nil {
		t.Fatalf("expected valid template, got %v", err)
	}
}

func TestPipelineTemplate_Validate_Invalid(t *testing.T) {
	cases := map[string]func(*PipelineTemplate){
		"empty id":         func(p *PipelineTemplate) { p.ID = "" },
		"blank id":         func(p *PipelineTemplate) { p.ID = "   " },
		"id too long":      func(p *PipelineTemplate) { p.ID = strings.Repeat("a", 101) },
		"empty name":       func(p *PipelineTemplate) { p.Name = "" },
		"name too long":    func(p *PipelineTemplate) { p.Name = strings.Repeat("n", 256) },
		"empty app type":   func(p *PipelineTemplate) { p.AppType = "" },
		"unknown app type": func(p *PipelineTemplate) { p.AppType = "web-backend" },
		"no stages":        func(p *PipelineTemplate) { p.Stages = nil },
		"blank stage":      func(p *PipelineTemplate) { p.Stages = []string{"Build", " "} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			tmpl := validTemplate()
			mutate(tmpl)
			err := tmpl.Validate()
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !errors.Is(err, ErrTemplateInvalid) {
				t.Fatalf("expected ErrTemplateInvalid, got %v", err)
			}
		})
	}
}

func TestAppType_IsValid(t *testing.T) {
	for _, at := range []AppType{AppTypeWeb, AppTypeBackend, AppTypeBatch} {
		if !at.IsValid() {
			t.Errorf("%q should be valid", at)
		}
	}
	for _, at := range []AppType{"", "web-backend", "WEB"} {
		if at.IsValid() {
			t.Errorf("%q should be invalid", at)
		}
	}
}
