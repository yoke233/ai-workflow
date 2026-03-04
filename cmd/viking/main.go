package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultOpenVikingBaseURL = "http://127.0.0.1:8088"
	defaultProbeTimeout      = 3 * time.Second
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	case "probe":
		return runProbe(args[1:], stdout, stderr)
	case "plan":
		return runPlan(args[1:], stdout, stderr)
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "viking - OpenViking helper for ai-workflow")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  viking probe [--base-url <url>] [--timeout <duration>]")
	fmt.Fprintln(w, "  viking plan --project <project-id> [--mode <mode>] [--env <env>] [--role <role>]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Modes:")
	fmt.Fprintln(w, "  chat | implement_backend | implement_frontend | review")
}

func runProbe(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(stderr)

	baseURL := defaultOpenVikingBaseURL
	timeout := defaultProbeTimeout
	fs.StringVar(&baseURL, "base-url", baseURL, "OpenViking base URL")
	fs.DurationVar(&timeout, "timeout", timeout, "HTTP timeout")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if timeout <= 0 {
		return errors.New("timeout must be > 0")
	}

	baseURL = normalizeBaseURL(baseURL)
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return fmt.Errorf("invalid base-url %q: %w", baseURL, err)
	}

	client := &http.Client{Timeout: timeout}
	paths := []string{"/health", "/api/health", "/v1/health"}

	okCount := 0
	for _, path := range paths {
		target := baseURL + path
		resp, err := client.Get(target)
		if err != nil {
			fmt.Fprintf(stdout, "[FAIL] GET %s -> %v\n", target, err)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		marker := "FAIL"
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			marker = "OK"
			okCount++
		}
		fmt.Fprintf(stdout, "[%s] GET %s -> %d\n", marker, target, resp.StatusCode)
	}

	if okCount == 0 {
		return fmt.Errorf("OpenViking probe failed: no healthy endpoint under %s", baseURL)
	}
	fmt.Fprintf(stdout, "OpenViking probe passed (%d/%d healthy)\n", okCount, len(paths))
	return nil
}

func runPlan(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)

	projectID := ""
	mode := "chat"
	env := "dev"
	role := "team_leader"
	fs.StringVar(&projectID, "project", projectID, "project id")
	fs.StringVar(&mode, "mode", mode, "execution mode")
	fs.StringVar(&env, "env", env, "environment name")
	fs.StringVar(&role, "role", role, "role name")

	if err := fs.Parse(args); err != nil {
		return err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return errors.New("project is required")
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	role = strings.ToLower(strings.TrimSpace(role))
	env = strings.ToLower(strings.TrimSpace(env))
	if mode == "" {
		mode = "chat"
	}
	if role == "" {
		role = "team_leader"
	}
	if env == "" {
		env = "dev"
	}

	uris := buildTargetURIs(projectID, mode)
	fmt.Fprintf(stdout, "project=%s mode=%s env=%s role=%s\n", projectID, mode, env, role)
	fmt.Fprintln(stdout, "target_uris:")
	for _, uri := range uris {
		fmt.Fprintf(stdout, "  - %s\n", uri)
	}

	if role == "team_leader" {
		fmt.Fprintln(stdout, "memory_policy: load+commit (writer)")
	} else {
		fmt.Fprintln(stdout, "memory_policy: load-only (reader)")
	}
	return nil
}

func buildTargetURIs(projectID string, mode string) []string {
	pid := strings.TrimSpace(projectID)
	mode = strings.ToLower(strings.TrimSpace(mode))

	uris := []string{
		"viking://resources/shared/",
		fmt.Sprintf("viking://resources/projects/%s/", pid),
		fmt.Sprintf("viking://memory/projects/%s/", pid),
	}

	switch mode {
	case "implement_backend":
		uris = append(uris, fmt.Sprintf("viking://resources/projects/%s/backend/", pid))
	case "implement_frontend":
		uris = append(uris, fmt.Sprintf("viking://resources/projects/%s/frontend/", pid))
	case "review":
		uris = append(uris, fmt.Sprintf("viking://resources/projects/%s/api/", pid))
	}

	return uniqStrings(uris)
}

func normalizeBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimSuffix(trimmed, "/")
	return trimmed
}

func uniqStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
