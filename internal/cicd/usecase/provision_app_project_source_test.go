package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cloud-nullus/draft/internal/cicd/adapter/scaffold"
	"github.com/cloud-nullus/draft/internal/cicd/domain"
	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// fakeSourceArchive 는 원본 아카이브 대역이다.
type fakeSourceArchive struct {
	files []port.CommitFile
	err   error
	calls []string
}

func (f *fakeSourceArchive) Supports(string) error { return nil }

func (f *fakeSourceArchive) Fetch(_ context.Context, repoURL, commit string) ([]port.CommitFile, error) {
	f.calls = append(f.calls, repoURL+"@"+commit)
	if f.err != nil {
		return nil, f.err
	}
	return f.files, nil
}

func egovBootSource() *domain.TemplateSource {
	return &domain.TemplateSource{
		RepoURL:      "https://github.com/eGovFramework/egovframe-boot-sample-java-config",
		Commit:       "c44e4bc8ea1e1b1e83e0aaab1037fbf04c761feb",
		Label:        "v5.0.0",
		BuildProfile: domain.BuildProfileSpringBoot,
	}
}

func egovBootArchive() *fakeSourceArchive {
	return &fakeSourceArchive{files: []port.CommitFile{
		{Path: "pom.xml", Content: "<project/>"},
		{Path: "Dockerfile", Content: "FROM maven:3.9-eclipse-temurin-17 AS build\n"},
		{Path: ".github/workflows/maven.yml", Content: "name: build-test\n"},
	}}
}

func sourceInput() ProvisionAppProjectInput {
	in := appInput()
	in.Source = egovBootSource()
	return in
}

func TestProvisionAppProject_ImportsSourceIntoNewRepository(t *testing.T) {
	scm, archive := newFakeSCM(), egovBootArchive()
	uc := NewProvisionAppProject(scm, newFakePipelineConfig(), gitlabResolver()).WithSourceArchive(archive)

	out, err := uc.Execute(context.Background(), sourceInput())
	require.NoError(t, err)

	assert.Equal(t, []string{
		"https://github.com/eGovFramework/egovframe-boot-sample-java-config@c44e4bc8ea1e1b1e83e0aaab1037fbf04c761feb",
	}, archive.calls)

	commits := scm.commits[out.Project.ID]
	require.Len(t, commits, 1, "원본과 플랫폼 파일이 한 커밋에 들어가야 첫 파이프라인이 온전한 저장소를 본다")
	files := map[string]string{}
	for _, f := range commits[0].Files {
		files[f.Path] = f.Content
	}
	assert.Equal(t, "<project/>", files["pom.xml"])
	assert.Contains(t, files[".gitlab-ci.yml"], "Nullus 가 생성한 파이프라인")
	assert.NotContains(t, files, ".github/workflows/maven.yml")
	assert.Contains(t, files, scaffold.PlatformGuidePath)

	assert.Contains(t, commits[0].Message, "egovframe-boot-sample-java-config@c44e4bc8ea1e")
	assert.Contains(t, commits[0].Message, "v5.0.0")
	require.NotNil(t, out.ImportedSource)
	assert.Equal(t, *egovBootSource(), *out.ImportedSource)
}

// 원본을 받지 못하면 방금 만든 저장소를 지운다. README 만 든 저장소가 남으면 같은 이름으로 다시 만들 때
// "이미 있던 저장소" 라 스캐폴딩을 건너뛰어, 빈 저장소를 가리키는 파이프라인이 생긴다.
func TestProvisionAppProject_SourceFetchFailureRollsBackNewRepository(t *testing.T) {
	scm := newFakeSCM()
	archive := &fakeSourceArchive{err: port.ErrSourceUnreachable}
	uc := NewProvisionAppProject(scm, newFakePipelineConfig(), gitlabResolver()).WithSourceArchive(archive)

	_, err := uc.Execute(context.Background(), sourceInput())
	require.Error(t, err)
	assert.True(t, errors.Is(err, port.ErrSourceUnreachable), "오프라인이면 인터넷이 필요하다고 알려야 한다: %v", err)
	assert.Equal(t, []string{"100"}, scm.deleted, "이번 호출이 만든 빈 저장소만 지운다")
	assert.Empty(t, scm.commits["100"])
}

func TestProvisionAppProject_SourceWithoutDockerfileRollsBackNewRepository(t *testing.T) {
	scm := newFakeSCM()
	archive := &fakeSourceArchive{files: []port.CommitFile{{Path: "pom.xml", Content: "<project/>"}}}
	in := sourceInput()
	in.Source.BuildProfile = domain.BuildProfileRepository
	uc := NewProvisionAppProject(scm, newFakePipelineConfig(), gitlabResolver()).WithSourceArchive(archive)

	_, err := uc.Execute(context.Background(), in)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Dockerfile")
	assert.Equal(t, []string{"100"}, scm.deleted)
	assert.Empty(t, scm.commits["100"])
}

// 원본을 올리는 커밋이 실패해도 같다. 원본 커밋은 스캐폴딩보다 훨씬 커서 SCM 이 거부하기 쉽다.
func TestProvisionAppProject_ImportCommitFailureRollsBackNewRepository(t *testing.T) {
	scm, archive := newFakeSCM(), egovBootArchive()
	scm.commitErr = errors.New("413 Request Entity Too Large")
	uc := NewProvisionAppProject(scm, newFakePipelineConfig(), gitlabResolver()).WithSourceArchive(archive)

	_, err := uc.Execute(context.Background(), sourceInput())
	require.Error(t, err)
	assert.Equal(t, []string{"100"}, scm.deleted)
}

// 지우지도 못하면 사람이 지워야 한다는 것을 오류에 담는다 — 남은 저장소 때문에 다음 시도가 엉뚱하게 돈다.
func TestProvisionAppProject_RollbackFailureIsReported(t *testing.T) {
	scm := newFakeSCM()
	scm.deleteErr = errors.New("403 Forbidden")
	archive := &fakeSourceArchive{err: port.ErrSourceUnreachable}
	uc := NewProvisionAppProject(scm, newFakePipelineConfig(), gitlabResolver()).WithSourceArchive(archive)

	_, err := uc.Execute(context.Background(), sourceInput())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acme/myapp", "지워야 할 저장소를 알려 준다")
	assert.True(t, errors.Is(err, port.ErrSourceUnreachable), "원래 원인은 그대로 남는다")
}

func TestProvisionAppProject_SourceWithoutArchiveWiringFails(t *testing.T) {
	scm := newFakeSCM()
	uc := NewProvisionAppProject(scm, newFakePipelineConfig(), gitlabResolver())

	_, err := uc.Execute(context.Background(), sourceInput())
	require.Error(t, err, "원본을 가져오겠다고 한 템플릿을 빈 스캐폴딩으로 만들면 nginx 가 배포된다")
	assert.Empty(t, scm.projects, "배선이 없는 것은 저장소를 만들기 전에 안다")
}

// 이미 있던 저장소에는 원본을 쓰지 않으므로 받지도 않는다. 오프라인 설치에서 기존 저장소로 파이프라인을
// 다시 만드는 길이 원본 호스트 때문에 막히면 안 된다.
func TestProvisionAppProject_ExistingRepositoryIgnoresSource(t *testing.T) {
	scm := newFakeSCM()
	scm.projectExists = true
	archive := &fakeSourceArchive{err: port.ErrSourceUnreachable}
	uc := NewProvisionAppProject(scm, newFakePipelineConfig(), gitlabResolver()).WithSourceArchive(archive)

	out, err := uc.Execute(context.Background(), sourceInput())
	require.NoError(t, err)

	assert.Empty(t, archive.calls)
	assert.Empty(t, scm.commits[out.Project.ID], "이미 있던 저장소에는 원본도 스캐폴딩도 쓰지 않는다")
	assert.Empty(t, scm.deleted, "있던 저장소는 절대 지우지 않는다")
	assert.True(t, out.ScaffoldSkipped)
	assert.Nil(t, out.ImportedSource, "가져오지 않았으면 가져왔다고 말하지 않는다")
}

func TestProvisionAppProject_RefreshDoesNotFetchSource(t *testing.T) {
	scm, archive := newFakeSCM(), egovBootArchive()
	scm.projectExists = true
	in := sourceInput()
	in.RefreshDefinitionOnly = true
	uc := NewProvisionAppProject(scm, newFakePipelineConfig(), gitlabResolver()).WithSourceArchive(archive)

	_, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Empty(t, archive.calls, "정의 갱신은 CI 정의 파일만 다시 쓴다 — 원본이 필요 없다")
}

// 정의를 갱신하려던 저장소가 없어 다시 만들면 처음처럼 원본을 다시 가져온다. 자리표시자 스캐폴딩으로
// 만들면 JVM 앱이 512Mi·probe 없는 매니페스트로 뜨고, 파이프라인 레코드는 원본을 가져왔다고 말한다.
func TestProvisionAppProject_RefreshRecreatedRepositoryReimportsSource(t *testing.T) {
	scm, archive := newFakeSCM(), egovBootArchive()
	in := sourceInput()
	in.RefreshDefinitionOnly = true
	uc := NewProvisionAppProject(scm, newFakePipelineConfig(), gitlabResolver()).WithSourceArchive(archive)

	out, err := uc.Execute(context.Background(), in)
	require.NoError(t, err)

	assert.True(t, out.RepositoryRecreated)
	assert.Len(t, archive.calls, 1)
	files := map[string]bool{}
	for _, f := range scm.commits[out.Project.ID][0].Files {
		files[f.Path] = true
	}
	assert.True(t, files["pom.xml"], "원본을 다시 가져왔다")
	require.NotNil(t, out.ImportedSource)
	joined := strings.Join(out.Warnings, "\n")
	assert.Contains(t, joined, "원본", "무엇을 다시 썼는지 알린다")
}
