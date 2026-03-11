export interface ParsedGitHubSkillSource {
  repoUrl: string;
  skillName: string;
}

export const parseGitHubSkillSource = (input: string): ParsedGitHubSkillSource | null => {
  const trimmed = input.trim();
  if (!trimmed) {
    return null;
  }

  const tokens = trimmed.split(/\s+/).filter(Boolean);
  const repoUrl = tokens.find((token) => /^https:\/\/github\.com\/[^/\s]+\/[^/\s]+(?:\.git)?$/i.test(token));
  if (!repoUrl) {
    return null;
  }

  const skillFlagIndex = tokens.findIndex((token) => token === "--skill");
  if (skillFlagIndex >= 0) {
    const skillName = tokens[skillFlagIndex + 1]?.trim();
    if (skillName) {
      return { repoUrl, skillName };
    }
  }

  const inlineSkill = tokens.find((token) => token.startsWith("--skill="));
  if (inlineSkill) {
    const skillName = inlineSkill.slice("--skill=".length).trim();
    if (skillName) {
      return { repoUrl, skillName };
    }
  }

  return null;
};
