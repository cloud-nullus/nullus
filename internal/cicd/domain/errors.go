package domain

import "fmt"

// ErrPipelineNotFound is returned when a pipeline cannot be found.
var ErrPipelineNotFound = fmt.Errorf("pipeline not found")

// ErrTemplateNotFound is returned when a pipeline template cannot be found.
var ErrTemplateNotFound = fmt.Errorf("pipeline template not found")

// ErrTemplateAlreadyExists 는 같은 ID 의 템플릿이 이미 있을 때 돌아온다.
var ErrTemplateAlreadyExists = fmt.Errorf("pipeline template already exists")

// ErrTemplateInvalid 는 템플릿이 저장 규칙을 어길 때 돌아온다. 어느 필드인지는 감싼 메시지에 있다.
var ErrTemplateInvalid = fmt.Errorf("pipeline template invalid")

// ErrDeploymentNotFound is returned when a deployment cannot be found.
var ErrDeploymentNotFound = fmt.Errorf("deployment not found")
