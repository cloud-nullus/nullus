package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// fakeSCM 은 호출을 기록하는 SCMProvisioner 대역이다.
type fakeSCM struct {
	groups        []port.GroupSpec
	projects      []port.ProjectSpec
	commits       map[string][]port.CommitSpec
	groupErr      error
	projectErr    error
	commitErr     error
	nextProjectID string
}

func newFakeSCM() *fakeSCM {
	return &fakeSCM{commits: map[string][]port.CommitSpec{}, nextProjectID: "100"}
}

func (f *fakeSCM) EnsureGroup(_ context.Context, spec port.GroupSpec) (*port.SCMGroup, error) {
	if f.groupErr != nil {
		return nil, f.groupErr
	}
	f.groups = append(f.groups, spec)
	return &port.SCMGroup{ID: "7", Name: spec.Name, FullPath: spec.Path}, nil
}

func (f *fakeSCM) EnsureProject(_ context.Context, spec port.ProjectSpec) (*port.SCMProject, error) {
	if f.projectErr != nil {
		return nil, f.projectErr
	}
	f.projects = append(f.projects, spec)
	full := spec.Path
	if spec.GroupPath != "" {
		full = spec.GroupPath + "/" + spec.Path
	}
	return &port.SCMProject{
		ID:            f.nextProjectID,
		Name:          spec.Name,
		FullPath:      full,
		RegistryURL:   "registry.test/" + full,
		DefaultBranch: "main",
		HTTPCloneURL:  "http://gl.test/" + full + ".git",
	}, nil
}

func (f *fakeSCM) CommitFiles(_ context.Context, projectID string, spec port.CommitSpec) error {
	if f.commitErr != nil {
		return f.commitErr
	}
	f.commits[projectID] = append(f.commits[projectID], spec)
	return nil
}


func TestProvisionCommonProject_CreatesGroupThenProject(t *testing.T) {
	scm := newFakeSCM()
	uc := NewProvisionCommonProject(scm)

	out, err := uc.Execute(context.Background(), ProvisionCommonProjectInput{
		GroupPath: "acme",
		GroupName: "Acme",
	})
	require.NoError(t, err)

	require.Len(t, scm.groups, 1)
	assert.Equal(t, "acme", scm.groups[0].Path)

	require.Len(t, scm.projects, 1)
	assert.Equal(t, DefaultCommonProjectPath, scm.projects[0].Path)
	assert.Equal(t, "7", scm.projects[0].GroupID)
	assert.Equal(t, "acme", scm.projects[0].GroupPath)

	assert.Equal(t, "acme/common", out.Project.FullPath)
	assert.Equal(t, "registry.test/acme/common", out.Project.RegistryURL)
}

// common 프로젝트 이름은 조직마다 다를 수 있어야 한다.
func TestProvisionCommonProject_HonorsCustomProjectPath(t *testing.T) {
	scm := newFakeSCM()
	uc := NewProvisionCommonProject(scm)

	_, err := uc.Execute(context.Background(), ProvisionCommonProjectInput{
		GroupPath:   "acme",
		ProjectPath: "platform-shared",
	})
	require.NoError(t, err)

	require.Len(t, scm.projects, 1)
	assert.Equal(t, "platform-shared", scm.projects[0].Path)
}

// 첫 커밋 전에 기본 브랜치가 없으면 파일 커밋이 실패한다.
func TestProvisionCommonProject_InitializesRepository(t *testing.T) {
	scm := newFakeSCM()
	uc := NewProvisionCommonProject(scm)

	_, err := uc.Execute(context.Background(), ProvisionCommonProjectInput{GroupPath: "acme"})
	require.NoError(t, err)

	require.Len(t, scm.projects, 1)
	assert.True(t, scm.projects[0].InitReadme, "기본 브랜치가 없으면 이후 커밋이 실패한다")
}

// common 은 베이스 이미지와 패키지를 담는 곳이므로, 무엇을 어떻게 올리는지
// 설명하는 문서를 함께 넣어 둔다. 빈 저장소는 용도를 알 수 없다.
func TestProvisionCommonProject_CommitsUsageDocument(t *testing.T) {
	scm := newFakeSCM()
	uc := NewProvisionCommonProject(scm)

	out, err := uc.Execute(context.Background(), ProvisionCommonProjectInput{GroupPath: "acme"})
	require.NoError(t, err)

	commits := scm.commits[out.Project.ID]
	require.Len(t, commits, 1)

	paths := map[string]string{}
	for _, f := range commits[0].Files {
		paths[f.Path] = f.Content
	}
	require.Contains(t, paths, "README.md")

	readme := paths["README.md"]
	assert.Contains(t, readme, "registry.test/acme/common", "베이스 이미지 경로가 문서에 있어야 한다")
	assert.Contains(t, readme, "npm")
	assert.Contains(t, readme, "maven")
}

func TestProvisionCommonProject_RequiresGroupPath(t *testing.T) {
	uc := NewProvisionCommonProject(newFakeSCM())

	_, err := uc.Execute(context.Background(), ProvisionCommonProjectInput{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "group_path")
}

func TestProvisionCommonProject_PropagatesProvisionerErrors(t *testing.T) {
	scm := newFakeSCM()
	scm.projectErr = errors.New("boom")
	uc := NewProvisionCommonProject(scm)

	_, err := uc.Execute(context.Background(), ProvisionCommonProjectInput{GroupPath: "acme"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

// 재실행돼도 같은 결과여야 한다 — Ensure* 가 멱등하므로 유스케이스도 멱등하다.
func TestProvisionCommonProject_IsRepeatable(t *testing.T) {
	scm := newFakeSCM()
	uc := NewProvisionCommonProject(scm)

	first, err := uc.Execute(context.Background(), ProvisionCommonProjectInput{GroupPath: "acme"})
	require.NoError(t, err)
	second, err := uc.Execute(context.Background(), ProvisionCommonProjectInput{GroupPath: "acme"})
	require.NoError(t, err)

	assert.Equal(t, first.Project.FullPath, second.Project.FullPath)
	assert.Equal(t, first.Project.RegistryURL, second.Project.RegistryURL)
}

