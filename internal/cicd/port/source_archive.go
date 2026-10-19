package port

import (
	"context"
	"errors"
)

// ErrSourceUnreachable 는 원본 저장소에 닿지 못했다는 뜻이다.
//
// 원본을 가져오는 템플릿은 인터넷 연결을 전제한다. 오프라인 설치에서 이 오류가 나면
// 그 템플릿을 쓸 수 없다는 뜻이지 일시적인 장애가 아니다.
var ErrSourceUnreachable = errors.New("원본 저장소에 닿지 못했습니다 — 원본을 가져오는 템플릿은 인터넷 연결이 필요합니다")

// ErrSourceNotFound 는 원본 저장소나 그 커밋이 없다는 뜻이다.
var ErrSourceNotFound = errors.New("원본 저장소 또는 커밋을 찾지 못했습니다")

// ErrSourceTooLarge 는 원본이 한 커밋으로 옮기기에 너무 크다는 뜻이다.
var ErrSourceTooLarge = errors.New("원본이 너무 커서 가져올 수 없습니다")

// ErrSourceUnsupported 는 가져올 수 없는 원본이라는 뜻이다 — 지원하지 않는 호스트·주소 모양이거나,
// 커밋 파일로 옮길 수 없는 항목(심볼릭 링크 등)이 있다. 템플릿을 고쳐야 낫는다.
var ErrSourceUnsupported = errors.New("가져올 수 없는 원본입니다")

// SourceArchive 는 외부 저장소의 특정 커밋 파일을 가져온다.
//
// 이력 없이 그 커밋의 파일만 돌려준다. 앱 저장소의 첫 커밋으로 그대로 올린다 — fork 는
// 같은 호스트 안에서만 되고(GitHub → 스택 GitLab 불가), 공개 저장소의 fork 는 private 로
// 둘 수 없다.
type SourceArchive interface {
	// Fetch 는 repoURL 저장소의 commit 시점 파일을 돌려준다. 경로는 저장소 루트 기준이다.
	Fetch(ctx context.Context, repoURL, commit string) ([]CommitFile, error)
	// Supports 는 그 주소를 가져올 수 있는 모양인지 본다(네트워크를 쓰지 않는다). 템플릿을 저장할 때
	// 부른다 — 저장은 되는데 파이프라인을 만들 때마다 실패하는 템플릿이 생기지 않게.
	Supports(repoURL string) error
}
