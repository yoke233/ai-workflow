package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func ApplyEnvOverrides(cfg *Config) error {
	if cfg == nil {
		return nil
	}

	if v, ok := os.LookupEnv("AI_WORKFLOW_SERVER_PORT"); ok {
		port, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("invalid AI_WORKFLOW_SERVER_PORT: %w", err)
		}
		cfg.Server.Port = port
	}
	if v, ok := os.LookupEnv("AI_WORKFLOW_SERVER_HOST"); ok {
		cfg.Server.Host = v
	}

	if v, ok := os.LookupEnv("AI_WORKFLOW_SCHEDULER_MAX_GLOBAL_AGENTS"); ok {
		maxAgents, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("invalid AI_WORKFLOW_SCHEDULER_MAX_GLOBAL_AGENTS: %w", err)
		}
		cfg.Scheduler.MaxGlobalAgents = maxAgents
	}
	if v, ok := os.LookupEnv("AI_WORKFLOW_SCHEDULER_WATCHDOG_ENABLED"); ok {
		enabled, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("invalid AI_WORKFLOW_SCHEDULER_WATCHDOG_ENABLED: %w", err)
		}
		cfg.Scheduler.Watchdog.Enabled = enabled
	}
	if v, ok := os.LookupEnv("AI_WORKFLOW_SCHEDULER_WATCHDOG_INTERVAL"); ok {
		duration, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("invalid AI_WORKFLOW_SCHEDULER_WATCHDOG_INTERVAL: %w", err)
		}
		cfg.Scheduler.Watchdog.Interval = Duration{Duration: duration}
	}
	if v, ok := os.LookupEnv("AI_WORKFLOW_SCHEDULER_WATCHDOG_STUCK_RUN_TTL"); ok {
		duration, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("invalid AI_WORKFLOW_SCHEDULER_WATCHDOG_STUCK_RUN_TTL: %w", err)
		}
		cfg.Scheduler.Watchdog.StuckRunTTL = Duration{Duration: duration}
	}
	if v, ok := os.LookupEnv("AI_WORKFLOW_SCHEDULER_WATCHDOG_STUCK_MERGE_TTL"); ok {
		duration, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("invalid AI_WORKFLOW_SCHEDULER_WATCHDOG_STUCK_MERGE_TTL: %w", err)
		}
		cfg.Scheduler.Watchdog.StuckMergeTTL = Duration{Duration: duration}
	}
	if v, ok := os.LookupEnv("AI_WORKFLOW_SCHEDULER_WATCHDOG_QUEUE_STALE_TTL"); ok {
		duration, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("invalid AI_WORKFLOW_SCHEDULER_WATCHDOG_QUEUE_STALE_TTL: %w", err)
		}
		cfg.Scheduler.Watchdog.QueueStaleTTL = Duration{Duration: duration}
	}

	if v, ok := os.LookupEnv("AI_WORKFLOW_GITHUB_TOKEN"); ok {
		cfg.GitHub.Token = v
	}

	return nil
}
