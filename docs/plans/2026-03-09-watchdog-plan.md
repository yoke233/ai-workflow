# Watchdog 巡检实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在 DepScheduler 内部引入 Watchdog 定时巡检 goroutine，检测并恢复卡死的 Run、滞留的 Issue 和泄漏的信号量。

**Architecture:** Watchdog 是 DepScheduler 的内部 goroutine，与现有 reconcileLoop 并行运行。通过 `WatchdogConfig` 配置阈值，`watchdogOnce()` 可独立测试。4 个巡检项：stuck_run、stuck_merging、queue_stale、sem_leak。

**Tech Stack:** Go 1.22+, SQLite, slog 日志

---

### Task 1: WatchdogConfig 配置结构

**Files:**
- Modify: `internal/config/types.go`
- Modify: `internal/config/defaults.toml`

**Step 1: 在 SchedulerConfig 中添加 WatchdogConfig**

在 `internal/config/types.go` 的 `SchedulerConfig` 结构体后面添加：

```go
type SchedulerConfig struct {
	MaxGlobalAgents int            `toml:"max_global_agents" yaml:"max_global_agents"`
	MaxProjectRuns  int            `toml:"max_project_runs"  yaml:"max_project_Runs"`
	Watchdog        WatchdogConfig `toml:"watchdog"          yaml:"watchdog"`
}

type WatchdogConfig struct {
	Enabled       bool     `toml:"enabled"         yaml:"enabled"`
	Interval      Duration `toml:"interval"        yaml:"interval"`
	StuckRunTTL   Duration `toml:"stuck_run_ttl"   yaml:"stuck_run_ttl"`
	StuckMergeTTL Duration `toml:"stuck_merge_ttl" yaml:"stuck_merge_ttl"`
	QueueStaleTTL Duration `toml:"queue_stale_ttl" yaml:"queue_stale_ttl"`
}
```

同时在 `SchedulerLayer` 后面添加：

```go
type SchedulerLayer struct {
	MaxGlobalAgents *int            `toml:"max_global_agents" yaml:"max_global_agents"`
	MaxProjectRuns  *int            `toml:"max_project_runs"  yaml:"max_project_Runs"`
	Watchdog        *WatchdogLayer  `toml:"watchdog"          yaml:"watchdog"`
}

type WatchdogLayer struct {
	Enabled       *bool     `toml:"enabled"         yaml:"enabled"`
	Interval      *Duration `toml:"interval"        yaml:"interval"`
	StuckRunTTL   *Duration `toml:"stuck_run_ttl"   yaml:"stuck_run_ttl"`
	StuckMergeTTL *Duration `toml:"stuck_merge_ttl" yaml:"stuck_merge_ttl"`
	QueueStaleTTL *Duration `toml:"queue_stale_ttl" yaml:"queue_stale_ttl"`
}
```

**Step 2: 在 defaults.toml 添加默认值**

在 `internal/config/defaults.toml` 的 `[scheduler]` 小节后追加：

```toml
  [scheduler.watchdog]
  enabled         = true
  interval        = "5m"
  stuck_run_ttl   = "30m"
  stuck_merge_ttl = "15m"
  queue_stale_ttl = "60m"
```

**Step 3: 验证编译**

Run: `go build ./internal/config/...`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add internal/config/types.go internal/config/defaults.toml
git commit -m "feat(config): add WatchdogConfig for scheduler health checks"
```

---

### Task 2: watchdog.go 核心逻辑

**Files:**
- Create: `internal/teamleader/watchdog.go`
- Modify: `internal/teamleader/scheduler.go` (添加字段)

**Step 1: 在 DepScheduler 添加 watchdog 字段**

在 `internal/teamleader/scheduler.go` 的 DepScheduler 结构体中，`reconcileWG` 后添加：

```go
	watchdogWG  sync.WaitGroup
```

**Step 2: 创建 watchdog.go**

```go
package teamleader

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/yoke233/ai-workflow/internal/config"
	"github.com/yoke233/ai-workflow/internal/core"
)

// watchdogDefaults returns a WatchdogConfig with sensible defaults applied
// for any zero-value fields.
func watchdogDefaults(cfg config.WatchdogConfig) config.WatchdogConfig {
	if cfg.Interval.Duration <= 0 {
		cfg.Interval.Duration = 5 * time.Minute
	}
	if cfg.StuckRunTTL.Duration <= 0 {
		cfg.StuckRunTTL.Duration = 30 * time.Minute
	}
	if cfg.StuckMergeTTL.Duration <= 0 {
		cfg.StuckMergeTTL.Duration = 15 * time.Minute
	}
	if cfg.QueueStaleTTL.Duration <= 0 {
		cfg.QueueStaleTTL.Duration = 60 * time.Minute
	}
	return cfg
}

// StartWatchdog launches the watchdog goroutine. It is safe to call multiple
// times; subsequent calls are no-ops while the watchdog is running.
func (s *DepScheduler) StartWatchdog(ctx context.Context, cfg config.WatchdogConfig) {
	if s == nil || !cfg.Enabled {
		return
	}
	cfg = watchdogDefaults(cfg)

	s.watchdogWG.Add(1)
	go func() {
		defer s.watchdogWG.Done()
		ticker := time.NewTicker(cfg.Interval.Duration)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.watchdogOnce(ctx, cfg)
			}
		}
	}()
	slog.Info("watchdog started",
		"interval", cfg.Interval.Duration,
		"stuck_run_ttl", cfg.StuckRunTTL.Duration,
		"stuck_merge_ttl", cfg.StuckMergeTTL.Duration,
		"queue_stale_ttl", cfg.QueueStaleTTL.Duration,
	)
}

// watchdogOnce performs a single inspection pass. Exported via method for testing.
func (s *DepScheduler) watchdogOnce(ctx context.Context, cfg config.WatchdogConfig) {
	s.checkStuckRuns(ctx, cfg.StuckRunTTL.Duration)
	s.checkStuckMerging(ctx, cfg.StuckMergeTTL.Duration)
	s.checkQueueStale(cfg.QueueStaleTTL.Duration)
	s.checkSemLeak()
}

// checkStuckRuns detects runs stuck in in_progress beyond the TTL threshold.
func (s *DepScheduler) checkStuckRuns(ctx context.Context, ttl time.Duration) {
	now := time.Now()
	var stuckRuns []string

	s.mu.Lock()
	for _, rs := range s.sessions {
		for issueID, runID := range rs.Running {
			issue := rs.IssueByID[issueID]
			if issue == nil || issue.Status != core.IssueStatusExecuting {
				continue
			}
			run, _ := s.store.GetRun(runID)
			if run == nil || run.Status != core.RunStatusInProgress {
				continue
			}
			if now.Sub(run.UpdatedAt) >= ttl {
				stuckRuns = append(stuckRuns, runID)
			}
		}
	}
	s.mu.Unlock()

	for _, runID := range stuckRuns {
		run, _ := s.store.GetRun(runID)
		age := time.Duration(0)
		if run != nil {
			age = now.Sub(run.UpdatedAt)
		}
		slog.Warn("watchdog: stuck run detected", "run_id", runID, "age", age)
		_ = s.OnEvent(ctx, core.Event{
			Type:      core.EventRunFailed,
			RunID:     runID,
			Error:     fmt.Sprintf("watchdog: run stuck for %v", age),
			Timestamp: now,
		})
	}
}

// checkStuckMerging detects issues stuck in merging status beyond the TTL.
func (s *DepScheduler) checkStuckMerging(ctx context.Context, ttl time.Duration) {
	now := time.Now()
	type stuckItem struct {
		issueID string
		runID   string
		age     time.Duration
	}
	var stuck []stuckItem

	s.mu.Lock()
	for _, rs := range s.sessions {
		for issueID := range rs.Running {
			issue := rs.IssueByID[issueID]
			if issue == nil || issue.Status != core.IssueStatusMerging {
				continue
			}
			if now.Sub(issue.UpdatedAt) >= ttl {
				stuck = append(stuck, stuckItem{
					issueID: issueID,
					runID:   issue.RunID,
					age:     now.Sub(issue.UpdatedAt),
				})
			}
		}
	}
	s.mu.Unlock()

	for _, item := range stuck {
		slog.Warn("watchdog: stuck merging detected", "issue_id", item.issueID, "age", item.age)
		_ = s.OnEvent(ctx, core.Event{
			Type:      core.EventRunFailed,
			RunID:     item.runID,
			IssueID:   item.issueID,
			Error:     fmt.Sprintf("watchdog: merging stuck for %v", item.age),
			Timestamp: now,
		})
	}
}

// checkQueueStale logs a warning for issues stuck in queued/ready beyond the TTL.
// No recovery action is taken — only alerting.
func (s *DepScheduler) checkQueueStale(ttl time.Duration) {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rs := range s.sessions {
		for issueID, issue := range rs.IssueByID {
			if issue == nil {
				continue
			}
			if issue.Status != core.IssueStatusQueued && issue.Status != core.IssueStatusReady {
				continue
			}
			if now.Sub(issue.UpdatedAt) >= ttl {
				slog.Warn("watchdog: stale queue item",
					"issue_id", issueID,
					"status", issue.Status,
					"age", now.Sub(issue.UpdatedAt),
				)
			}
		}
	}
}

// checkSemLeak detects and recovers leaked semaphore slots.
func (s *DepScheduler) checkSemLeak() {
	s.mu.Lock()
	defer s.mu.Unlock()

	semUsed := len(s.sem)
	actualRunning := 0
	for _, rs := range s.sessions {
		actualRunning += len(rs.Running)
	}

	if semUsed > actualRunning {
		leaked := semUsed - actualRunning
		slog.Warn("watchdog: semaphore leak detected",
			"sem_used", semUsed,
			"actual_running", actualRunning,
			"leaked", leaked,
		)
		for i := 0; i < leaked; i++ {
			s.releaseSlot()
		}
	}
}
```

**Step 3: 验证编译**

Run: `go build ./internal/teamleader/...`
Expected: BUILD SUCCESS

**Step 4: Commit**

```bash
git add internal/teamleader/watchdog.go internal/teamleader/scheduler.go
git commit -m "feat(watchdog): add watchdog goroutine with 4 health checks"
```

---

### Task 3: 集成 Watchdog 到 Scheduler 生命周期

**Files:**
- Modify: `internal/teamleader/scheduler.go` (Start/Stop 方法)

**Step 1: 在 DepScheduler 添加 watchdog 配置字段**

在 `internal/teamleader/scheduler.go` 的 DepScheduler 结构体中添加：

```go
	watchdogCfg config.WatchdogConfig
```

**Step 2: 添加 SetWatchdogConfig 方法**

在 `SetReconcileRunner` 方法后添加：

```go
// SetWatchdogConfig configures the watchdog health check parameters.
func (s *DepScheduler) SetWatchdogConfig(cfg config.WatchdogConfig) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.watchdogCfg = cfg
}
```

**Step 3: 修改 Start 方法，启动 watchdog**

在 `Start()` 方法中，`reconcileRun` goroutine 启动代码块之后（`return nil` 之前），添加：

```go
	if s.watchdogCfg.Enabled {
		s.StartWatchdog(runCtx, s.watchdogCfg)
	}
```

**Step 4: 修改 Stop 方法，等待 watchdog 退出**

在 `Stop()` 方法的 `go func()` 内，`s.reconcileWG.Wait()` 后面添加：

```go
		s.watchdogWG.Wait()
```

**Step 5: 验证编译**

Run: `go build ./internal/teamleader/...`
Expected: BUILD SUCCESS

**Step 6: Commit**

```bash
git add internal/teamleader/scheduler.go
git commit -m "feat(watchdog): integrate watchdog into scheduler Start/Stop lifecycle"
```

---

### Task 4: Watchdog 单元测试

**Files:**
- Create: `internal/teamleader/watchdog_test.go`

**Step 1: 写 stuck_run 检测测试**

```go
package teamleader

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yoke233/ai-workflow/internal/config"
	"github.com/yoke233/ai-workflow/internal/core"
)

func TestWatchdog_StuckRunRecovery(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()
	project := mustCreateSchedulerProject(t, store, "proj-wd-stuck-run")

	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-wd-1", core.FailSkip, []core.Issue{
		newIssueWithProfile("issue-wd-stuck", "stuck run test", core.WorkflowProfileStrict, nil),
	})

	var mu sync.Mutex
	runCalled := false
	blockingRunner := func(_ context.Context, _ string) error {
		mu.Lock()
		runCalled = true
		mu.Unlock()
		// Block forever to simulate stuck run.
		select {}
	}

	s := NewDepScheduler(store, nil, blockingRunner, nil, 2)
	if err := s.ScheduleIssues(context.Background(), issues); err != nil {
		t.Fatalf("ScheduleIssues() error = %v", err)
	}

	// Wait for the run to start.
	waitIssueStatus(t, store, "issue-wd-stuck", core.IssueStatusExecuting, 3*time.Second)

	// Run watchdog with a very short TTL so the run is immediately "stuck".
	cfg := config.WatchdogConfig{
		Enabled:       true,
		StuckRunTTL:   config.Duration{Duration: 1 * time.Millisecond},
		StuckMergeTTL: config.Duration{Duration: 1 * time.Hour},
		QueueStaleTTL: config.Duration{Duration: 1 * time.Hour},
	}
	s.watchdogOnce(context.Background(), cfg)

	// After watchdog fires EventRunFailed, the issue should transition to failed.
	waitIssueStatus(t, store, "issue-wd-stuck", core.IssueStatusFailed, 3*time.Second)
}

func TestWatchdog_SemLeakRecovery(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()

	s := NewDepScheduler(store, nil, nil, nil, 3)

	// Manually leak 2 slots.
	s.sem <- struct{}{}
	s.sem <- struct{}{}

	if len(s.sem) != 2 {
		t.Fatalf("expected 2 leaked slots, got %d", len(s.sem))
	}

	cfg := config.WatchdogConfig{
		Enabled:       true,
		StuckRunTTL:   config.Duration{Duration: 1 * time.Hour},
		StuckMergeTTL: config.Duration{Duration: 1 * time.Hour},
		QueueStaleTTL: config.Duration{Duration: 1 * time.Hour},
	}
	s.watchdogOnce(context.Background(), cfg)

	if len(s.sem) != 0 {
		t.Fatalf("expected 0 slots used after leak recovery, got %d", len(s.sem))
	}
}

func TestWatchdog_QueueStaleOnlyLogs(t *testing.T) {
	store := newSchedulerTestStore(t)
	defer store.Close()
	project := mustCreateSchedulerProject(t, store, "proj-wd-stale")

	issues := mustCreateIssueSessionWithItems(t, store, project.ID, "session-wd-2", core.FailSkip, []core.Issue{
		newIssueWithProfile("issue-wd-stale", "stale queue test", core.WorkflowProfileStrict, nil),
	})

	// Use a no-op runner so nothing ever dispatches (sem capacity 0 would also work).
	s := NewDepScheduler(store, nil, nil, nil, 0)
	// Register session without dispatching (capacity 0 blocks).
	_ = s.ScheduleIssues(context.Background(), issues)

	cfg := config.WatchdogConfig{
		Enabled:       true,
		StuckRunTTL:   config.Duration{Duration: 1 * time.Hour},
		StuckMergeTTL: config.Duration{Duration: 1 * time.Hour},
		QueueStaleTTL: config.Duration{Duration: 1 * time.Millisecond},
	}

	// Should not panic or error — only log.
	s.watchdogOnce(context.Background(), cfg)

	// Issue should NOT change status (stale queue only logs).
	issue, _ := store.GetIssue("issue-wd-stale")
	if issue == nil {
		t.Fatal("issue not found")
	}
	if issue.Status == core.IssueStatusFailed {
		t.Fatal("queue stale should only log, not fail the issue")
	}
}
```

**Step 2: 运行测试**

Run: `go test ./internal/teamleader/... -run TestWatchdog -v -timeout 30s`
Expected: 3 tests PASS

**Step 3: Commit**

```bash
git add internal/teamleader/watchdog_test.go
git commit -m "test(watchdog): add unit tests for stuck run, sem leak, and stale queue"
```

---

### Task 5: 在启动链路中注入 WatchdogConfig

**Files:**
- Modify: `cmd/ai-flow/commands.go` 或启动入口文件中调用 `SetWatchdogConfig` 的位置

**Step 1: 找到 DepScheduler 创建位置**

在 `cmd/ai-flow/commands.go` 或 `internal/teamleader/manager.go` 中找到调用 `NewDepScheduler(...)` 的代码，在其后添加：

```go
scheduler.SetWatchdogConfig(cfg.Scheduler.Watchdog)
```

其中 `cfg` 是 `config.Config`。

**Step 2: 验证编译**

Run: `go build ./cmd/ai-flow/...`
Expected: BUILD SUCCESS

**Step 3: 运行全部 scheduler 测试**

Run: `go test ./internal/teamleader/... -timeout 60s`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add cmd/ai-flow/commands.go
git commit -m "feat(watchdog): wire WatchdogConfig into scheduler startup"
```

---

### Task 6: 运行完整后端测试套件

**Step 1: 运行全部后端测试**

Run: `pwsh -NoProfile -File ./scripts/test/backend-all.ps1`
Expected: ALL PASS

**Step 2: 如有失败，修复并重新运行**

**Step 3: Commit（如有修复）**

```bash
git commit -m "fix(watchdog): address test failures from integration"
```
