import { describe, expect, it } from "vitest";
import { parseGitHubSkillSource } from "./skills";

describe("parseGitHubSkillSource", () => {
  it("parses npm install command style input", () => {
    expect(parseGitHubSkillSource(
      "npx skills add https://github.com/vercel-labs/agent-skills --skill vercel-react-best-practices",
    )).toEqual({
      repoUrl: "https://github.com/vercel-labs/agent-skills",
      skillName: "vercel-react-best-practices",
    });
  });

  it("parses inline --skill syntax", () => {
    expect(parseGitHubSkillSource(
      "npx skills add https://github.com/vercel-labs/agent-skills --skill=vercel-react-best-practices",
    )).toEqual({
      repoUrl: "https://github.com/vercel-labs/agent-skills",
      skillName: "vercel-react-best-practices",
    });
  });

  it("returns null when repo url is missing", () => {
    expect(parseGitHubSkillSource("--skill vercel-react-best-practices")).toBeNull();
  });

  it("returns null when skill flag is missing", () => {
    expect(parseGitHubSkillSource("https://github.com/vercel-labs/agent-skills")).toBeNull();
  });
});
