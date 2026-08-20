package docker

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 운영 배포 로그에 "error:" 한 줄만 남고 끝난 적이 있다. CombinedOutput 은 실행
// 파일 자체가 없으면 출력 없이 에러만 돌려주는데, 호출부가 출력만 찍었기 때문이다.
// 무엇이 없어서 실패했는지는 err 에만 들어 있다.
func TestCommandFailure_NamesTheMissingTool(t *testing.T) {
	err := &exec.Error{Name: "git", Err: exec.ErrNotFound}

	msg := commandFailure("git", nil, err)

	assert.Contains(t, msg, "git")
	assert.Contains(t, msg, "설치")
	assert.NotEqual(t, "", msg)
}

func TestCommandFailure_PrefersCommandOutput(t *testing.T) {
	output := []byte("fatal: repository 'https://gitea.nullus.io/root/spring-sample.git' not found\n")

	msg := commandFailure("git", output, errors.New("exit status 128"))

	assert.Equal(t, "fatal: repository 'https://gitea.nullus.io/root/spring-sample.git' not found", msg)
}

func TestCommandFailure_FallsBackToError(t *testing.T) {
	msg := commandFailure("docker", []byte("   \n"), errors.New("exit status 1"))

	assert.Equal(t, "exit status 1", msg)
}
