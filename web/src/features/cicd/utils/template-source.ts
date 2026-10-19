import type { TemplateSource } from "../../../types";

const SHORT_COMMIT_LENGTH = 12;

export interface DescribedTemplateSource {
  // repository 는 화면에 보일 저장소 이름이다(github.com 이면 소유자/저장소).
  repository: string;
  // version 은 원본 버전 이름이다. 없으면 짧은 커밋이다 — 가져오는 대상은 늘 커밋이다.
  version: string;
  shortCommit: string;
  url: string;
}

// describeTemplateSource 는 템플릿 원본을 카드와 생성 화면이 같은 말로 보이게 옮긴다.
export function describeTemplateSource(
  source: TemplateSource,
): DescribedTemplateSource {
  const shortCommit = source.commit.slice(0, SHORT_COMMIT_LENGTH);
  return {
    repository: repositoryName(source.repoUrl),
    version: source.label?.trim() || shortCommit,
    shortCommit,
    url: source.repoUrl,
  };
}

function repositoryName(repoUrl: string): string {
  try {
    const path = new URL(repoUrl).pathname.replace(/^\/+|\/+$/g, "");
    return path.replace(/\.git$/, "") || repoUrl;
  } catch {
    return repoUrl;
  }
}
