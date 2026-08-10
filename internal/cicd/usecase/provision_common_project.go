package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloud-nullus/draft/internal/cicd/port"
)

// DefaultCommonProjectPath 는 공용 프로젝트의 기본 경로다.
//
// GitLab 의 Container/Package Registry 는 프로젝트 종속이라, 조직 공용 베이스
// 이미지와 npm/maven 패키지를 두려면 그것을 소유할 프로젝트가 하나 필요하다.
// 이름은 조직 관례에 따라 바꿀 수 있다.
const DefaultCommonProjectPath = "common"

// ProvisionCommonProjectInput 은 공용 프로젝트 프로비저닝 요청이다.
type ProvisionCommonProjectInput struct {
	// GroupPath 는 조직 그룹 경로다 (예: "acme").
	GroupPath string
	// GroupName 은 그룹 표시 이름이다. 비면 GroupPath 를 쓴다.
	GroupName string
	// ProjectPath 는 공용 프로젝트 경로다. 비면 DefaultCommonProjectPath.
	ProjectPath string
	// Platform 은 안내 문서에 적을 패키지 레지스트리 주소 형식을 정한다.
	// 비면 GitLab 으로 본다.
	Platform port.SCMPlatform
}

// ProvisionCommonProjectOutput 은 프로비저닝 결과다.
type ProvisionCommonProjectOutput struct {
	Group   *port.SCMGroup
	Project *port.SCMProject
}

// ProvisionCommonProject 는 조직 그룹과 공용 프로젝트를 보장한다.
type ProvisionCommonProject struct {
	scm port.SCMProvisioner
}

// NewProvisionCommonProject 는 유스케이스를 만든다.
func NewProvisionCommonProject(scm port.SCMProvisioner) *ProvisionCommonProject {
	return &ProvisionCommonProject{scm: scm}
}

// Execute 는 그룹 → 공용 프로젝트 → 사용 안내 문서 순으로 보장한다.
//
// 모든 단계가 멱등하므로 스택 설치가 재시도돼도 안전하다.
func (uc *ProvisionCommonProject) Execute(
	ctx context.Context,
	input ProvisionCommonProjectInput,
) (*ProvisionCommonProjectOutput, error) {
	groupPath := strings.TrimSpace(input.GroupPath)
	if groupPath == "" {
		return nil, fmt.Errorf("group_path is required")
	}
	projectPath := strings.TrimSpace(input.ProjectPath)
	if projectPath == "" {
		projectPath = DefaultCommonProjectPath
	}

	group, err := uc.scm.EnsureGroup(ctx, port.GroupSpec{
		Name:        firstNonEmptyString(input.GroupName, groupPath),
		Path:        groupPath,
		Description: "Nullus 가 관리하는 조직 그룹",
	})
	if err != nil {
		return nil, fmt.Errorf("ensure group %q: %w", groupPath, err)
	}

	project, err := uc.scm.EnsureProject(ctx, port.ProjectSpec{
		Name:        projectPath,
		Path:        projectPath,
		GroupID:     group.ID,
		GroupPath:   group.FullPath,
		Description: "공용 베이스 이미지와 npm/maven 패키지 레지스트리",
		Visibility:  "private",
		// 기본 브랜치가 없으면 이후 파일 커밋이 실패한다.
		InitReadme: true,
	})
	if err != nil {
		return nil, fmt.Errorf("ensure common project %q: %w", projectPath, err)
	}

	commit := port.CommitSpec{
		Branch:  project.DefaultBranch,
		Message: "docs(common): describe shared registry usage",
		Files: []port.CommitFile{
			{Path: "README.md", Content: commonProjectReadme(input.Platform, project)},
		},
	}
	if err := uc.scm.CommitFiles(ctx, project.ID, commit); err != nil {
		return nil, fmt.Errorf("commit common project docs: %w", err)
	}

	return &ProvisionCommonProjectOutput{Group: group, Project: project}, nil
}

// commonProjectReadme 는 공용 레지스트리 사용법을 설명한다.
//
// 빈 저장소는 용도를 알 수 없으므로 경로가 박힌 문서를 함께 둔다.
// 패키지 레지스트리 주소는 플랫폼마다 다르므로 여기서 갈라 쓴다 — GitLab 형식을
// GitHub 리포에 적어 두면 사람이 그대로 따라 하다 실패한다.
func commonProjectReadme(platform port.SCMPlatform, project *port.SCMProject) string {
	registry := project.RegistryURL
	if strings.TrimSpace(registry) == "" {
		registry = "<registry-host>/" + project.FullPath
	}

	var b strings.Builder
	b.WriteString("# " + project.FullPath + "\n\n")
	b.WriteString("Nullus 가 관리하는 **공용 프로젝트**입니다. 조직이 공유하는 베이스 이미지와\n")
	b.WriteString("패키지를 이곳의 레지스트리에 둡니다. 애플리케이션 프로젝트는 여기를 참조합니다.\n\n")

	b.WriteString("## 컨테이너 베이스 이미지\n\n")
	b.WriteString("```bash\n")
	b.WriteString("docker build -t " + registry + "/<이미지>:<태그> .\n")
	b.WriteString("docker push " + registry + "/<이미지>:<태그>\n")
	b.WriteString("```\n\n")
	b.WriteString("애플리케이션 Dockerfile 에서:\n\n")
	b.WriteString("```dockerfile\n")
	b.WriteString("FROM " + registry + "/<이미지>:<태그>\n")
	b.WriteString("```\n\n")

	if platform == port.SCMPlatformGitHub {
		writeGitHubPackageDocs(&b, project)
	} else {
		writeGitLabPackageDocs(&b, project)
	}

	b.WriteString("---\n\n")
	b.WriteString("이 파일은 Nullus 가 생성했습니다. 직접 편집해도 되지만, 프로비저닝이\n")
	b.WriteString("다시 실행되면 덮어써집니다.\n")
	return b.String()
}

func writeGitLabPackageDocs(b *strings.Builder, project *port.SCMProject) {
	b.WriteString("## npm 패키지 레지스트리\n\n")
	b.WriteString("```bash\n")
	b.WriteString("npm config set @<스코프>:registry " + project.WebURL + "/-/packages/npm/\n")
	b.WriteString("```\n\n")

	b.WriteString("## maven 패키지 레지스트리\n\n")
	b.WriteString("`pom.xml` 의 `<distributionManagement>` 에 이 프로젝트의 Package Registry\n")
	b.WriteString("주소를 등록하면 됩니다.\n\n")
}

// writeGitHubPackageDocs 는 GitHub Packages 주소를 적는다.
//
// GitHub 은 생태계마다 호스트가 다르다(npm.pkg.github.com, maven.pkg.github.com).
// GitLab 처럼 프로젝트 URL 아래 경로가 아니므로 형식을 따로 적어야 한다.
func writeGitHubPackageDocs(b *strings.Builder, project *port.SCMProject) {
	owner := project.FullPath
	if idx := strings.Index(owner, "/"); idx > 0 {
		owner = owner[:idx]
	}

	b.WriteString("## npm 패키지 레지스트리\n\n")
	b.WriteString("```bash\n")
	b.WriteString("npm config set @" + owner + ":registry https://npm.pkg.github.com\n")
	b.WriteString("```\n\n")

	b.WriteString("## maven 패키지 레지스트리\n\n")
	b.WriteString("`pom.xml` 의 `<distributionManagement>` 에 아래 주소를 등록하면 됩니다.\n\n")
	b.WriteString("```\n")
	b.WriteString("https://maven.pkg.github.com/" + project.FullPath + "\n")
	b.WriteString("```\n\n")
}

func firstNonEmptyString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
