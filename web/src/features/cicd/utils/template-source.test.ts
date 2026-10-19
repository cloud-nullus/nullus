import { describe, expect, it } from "vitest";
import { describeTemplateSource } from "./template-source";

describe("describeTemplateSource", () => {
  it("should show the GitHub owner/repository and the release label", () => {
    expect(
      describeTemplateSource({
        repoUrl: "https://github.com/eGovFramework/egovframe-web-sample",
        commit: "158daebd2bfdab951e90ad526c3d2a6b6b42513b",
        label: "v5.0.0",
        buildProfile: "maven-war",
      }),
    ).toEqual({
      repository: "eGovFramework/egovframe-web-sample",
      version: "v5.0.0",
      shortCommit: "158daebd2bfd",
      url: "https://github.com/eGovFramework/egovframe-web-sample",
    });
  });

  it("should fall back to the short commit when the source has no label", () => {
    const described = describeTemplateSource({
      repoUrl: "https://github.com/acme/app.git",
      commit: "c44e4bc8ea1e1b1e83e0aaab1037fbf04c761feb",
    });
    expect(described.repository).toBe("acme/app");
    expect(described.version).toBe("c44e4bc8ea1e");
  });

  it("should keep an unparsable address as it is", () => {
    expect(
      describeTemplateSource({ repoUrl: "not a url", commit: "abc" })
        .repository,
    ).toBe("not a url");
  });
});
