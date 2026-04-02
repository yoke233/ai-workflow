package appcmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yoke233/zhanggui/internal/core"
	"github.com/yoke233/zhanggui/internal/platform/config"
)

func TestRunProfileListSeedsMinimumOrg(t *testing.T) {
	t.Setenv("AI_WORKFLOW_DATA_DIR", t.TempDir())

	runtime, err := defaultNewProfileRuntime()
	if err != nil {
		t.Fatalf("defaultNewProfileRuntime() error = %v", err)
	}
	defer runtime.close()

	var stdout bytes.Buffer
	if err := runProfileToWriter(&stdout, runtime, []string{"list"}); err != nil {
		t.Fatalf("runProfileToWriter(list) error = %v", err)
	}

	var result profileResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.OK {
		t.Fatalf("result.OK = false: %+v", result)
	}
	if len(result.List) != 4 {
		t.Fatalf("len(result.List) = %d, want 4", len(result.List))
	}
	ids := make([]string, 0, len(result.List))
	for _, profile := range result.List {
		ids = append(ids, profile.ID)
	}
	if got, want := strings.Join(ids, ","), "ceo,lead,reviewer,worker"; got != want {
		t.Fatalf("profile ids = %q, want %q", got, want)
	}
}

func TestRunProfileCreateMutateAndDelete(t *testing.T) {
	t.Setenv("AI_WORKFLOW_DATA_DIR", t.TempDir())

	runtime, err := defaultNewProfileRuntime()
	if err != nil {
		t.Fatalf("defaultNewProfileRuntime() error = %v", err)
	}
	defer runtime.close()

	create := executeProfileTestCommand(t, runtime, []string{
		"create",
		"--from", "ceo",
		"--id", "dev1",
		"--name", "Dev One",
		"--role", "worker",
		"--driver", "codex-acp",
	})
	if create.Profile == nil {
		t.Fatal("create.Profile = nil")
	}
	if create.Profile.ManagerProfileID != "ceo" {
		t.Fatalf("create.Profile.ManagerProfileID = %q, want ceo", create.Profile.ManagerProfileID)
	}
	if create.Profile.DriverID != "codex-acp" {
		t.Fatalf("create.Profile.DriverID = %q, want codex-acp", create.Profile.DriverID)
	}
	if create.Profile.PromptTemplate != "implement" {
		t.Fatalf("create.Profile.PromptTemplate = %q, want implement", create.Profile.PromptTemplate)
	}
	if create.Profile.Role != core.RoleWorker {
		t.Fatalf("create.Profile.Role = %q, want worker", create.Profile.Role)
	}

	cfg, _, _, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if !configHasProfile(cfg, "dev1") {
		t.Fatal("expected config to contain dev1 after create")
	}

	addSkill := executeProfileTestCommand(t, runtime, []string{"add-skill", "--id", "dev1", "--skill", "plan-actions"})
	if addSkill.Profile == nil {
		t.Fatal("addSkill.Profile = nil")
	}
	if !hasSkill(addSkill.Profile, "plan-actions") {
		t.Fatalf("expected plan-actions in skills, got %#v", addSkill.Profile.Skills)
	}

	setBase := executeProfileTestCommand(t, runtime, []string{
		"set-base",
		"--id", "dev1",
		"--name", "Dev Prime",
		"--role", "gate",
		"--driver", "claude-acp",
		"--prompt", "custom_review",
	})
	if setBase.Profile == nil {
		t.Fatal("setBase.Profile = nil")
	}
	if setBase.Profile.Name != "Dev Prime" {
		t.Fatalf("setBase.Profile.Name = %q, want Dev Prime", setBase.Profile.Name)
	}
	if setBase.Profile.Role != core.RoleGate {
		t.Fatalf("setBase.Profile.Role = %q, want gate", setBase.Profile.Role)
	}
	if setBase.Profile.DriverID != "claude-acp" {
		t.Fatalf("setBase.Profile.DriverID = %q, want claude-acp", setBase.Profile.DriverID)
	}
	if setBase.Profile.PromptTemplate != "custom_review" {
		t.Fatalf("setBase.Profile.PromptTemplate = %q, want custom_review", setBase.Profile.PromptTemplate)
	}
	if !strings.Contains(strings.Join(setBase.Profile.Capabilities, ","), "review") {
		t.Fatalf("expected review capability after role change, got %#v", setBase.Profile.Capabilities)
	}

	removeSkill := executeProfileTestCommand(t, runtime, []string{"remove-skill", "--id", "dev1", "--skill", "plan-actions"})
	if removeSkill.Profile == nil {
		t.Fatal("removeSkill.Profile = nil")
	}
	if hasSkill(removeSkill.Profile, "plan-actions") {
		t.Fatalf("expected plan-actions removed, got %#v", removeSkill.Profile.Skills)
	}

	get := executeProfileTestCommand(t, runtime, []string{"get", "--id", "dev1"})
	if get.Profile == nil {
		t.Fatal("get.Profile = nil")
	}
	if get.Profile.Name != "Dev Prime" {
		t.Fatalf("get.Profile.Name = %q, want Dev Prime", get.Profile.Name)
	}

	executeProfileTestCommand(t, runtime, []string{"delete", "--id", "dev1"})

	cfg, _, _, err = LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig(reload) error = %v", err)
	}
	if configHasProfile(cfg, "dev1") {
		t.Fatal("expected config to remove dev1 after delete")
	}

	list := executeProfileTestCommand(t, runtime, []string{"list"})
	if len(list.List) != 4 {
		t.Fatalf("expected minimum org after delete, got %#v", list.List)
	}
	if got := strings.Join([]string{list.List[0].ID, list.List[1].ID, list.List[2].ID, list.List[3].ID}, ","); got != "ceo,lead,reviewer,worker" {
		t.Fatalf("unexpected profile ids after delete = %q", got)
	}
}

func TestRunProfileDeleteRejectsCEO(t *testing.T) {
	t.Setenv("AI_WORKFLOW_DATA_DIR", t.TempDir())

	runtime, err := defaultNewProfileRuntime()
	if err != nil {
		t.Fatalf("defaultNewProfileRuntime() error = %v", err)
	}
	defer runtime.close()

	var stdout bytes.Buffer
	err = runProfileToWriter(&stdout, runtime, []string{"delete", "--id", "ceo"})
	if err == nil {
		t.Fatal("expected delete ceo to fail")
	}
	if !strings.Contains(err.Error(), "protected") {
		t.Fatalf("unexpected error = %v", err)
	}
}

func executeProfileTestCommand(t *testing.T, runtime *profileRuntime, args []string) profileResult {
	t.Helper()

	var stdout bytes.Buffer
	if err := runProfileToWriter(&stdout, runtime, args); err != nil {
		t.Fatalf("runProfileToWriter(%v) error = %v", args, err)
	}

	var result profileResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal result for %v: %v", args, err)
	}
	if !result.OK {
		t.Fatalf("result.OK = false for %v: %+v", args, result)
	}
	return result
}

func configHasProfile(cfg *config.Config, id string) bool {
	if cfg == nil {
		return false
	}
	for _, item := range cfg.Runtime.Agents.Profiles {
		if item.ID == id {
			return true
		}
	}
	return false
}

func hasSkill(profile *core.AgentProfile, skill string) bool {
	if profile == nil {
		return false
	}
	for _, item := range profile.Skills {
		if item == skill {
			return true
		}
	}
	return false
}
