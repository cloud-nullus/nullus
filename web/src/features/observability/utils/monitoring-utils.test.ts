import { beforeEach, describe, expect, it } from "vitest";

import {
  SKIP_KEY,
  STORAGE_KEY,
  loadTabs,
  persistTabs,
  type EmbedTab,
} from "./monitoring-utils";

const tab = (label: string, url: string): EmbedTab => ({
  id: `tab-${label}`,
  label,
  url,
  order: 0,
});

describe("모니터링 탭 저장소", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  // 탭에는 스택 접속 도메인에서 나온 주소가 담긴다. 키가 뷰 단위면 스택 A 의
  // 주소가 스택 B 화면에 그대로 뜬다.
  it("저장 키를 스코프별로 나눈다", () => {
    expect(STORAGE_KEY("stack", "stack-a")).not.toBe(STORAGE_KEY("stack", "stack-b"));
    expect(SKIP_KEY("stack", "stack-a")).not.toBe(SKIP_KEY("stack", "stack-b"));
  });

  it("한 스택에 저장한 탭이 다른 스택에 새어 나가지 않는다", () => {
    persistTabs("stack", "stack-a", [tab("Grafana", "https://grafana.a.local")]);

    expect(loadTabs("stack", "stack-a")).toHaveLength(1);
    expect(loadTabs("stack", "stack-b")).toEqual([]);
  });

  it("시드 탭은 그 스코프에만 심는다", () => {
    const seed = [tab("Pipelines", "https://ci.a.local")];

    expect(loadTabs("cicd", "cluster-a", seed)).toEqual(seed);
    expect(loadTabs("cicd", "cluster-b")).toEqual([]);
  });

  it("저장된 탭이 있으면 시드로 덮어쓰지 않는다", () => {
    persistTabs("cicd", "cluster-a", [tab("Mine", "https://mine.local")]);

    expect(loadTabs("cicd", "cluster-a", [tab("Seed", "https://seed.local")])).toEqual([
      tab("Mine", "https://mine.local"),
    ]);
  });
});
