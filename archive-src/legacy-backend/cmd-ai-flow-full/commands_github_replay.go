package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ghwebhook "github.com/yoke233/ai-workflow/internal/github"
)

var githubReplayByDeliveryID = func(ctx context.Context, deliveryID string) (bool, error) {
	dispatcher := ghwebhook.NewWebhookDispatcher(ghwebhook.WebhookDispatcherOptions{
		DLQStore: ghwebhook.DefaultDLQStore(),
		Handler: ghwebhook.WebhookDispatchHandlerFunc(func(context.Context, ghwebhook.WebhookDispatchRequest) error {
			return nil
		}),
	})
	defer dispatcher.Close()
	return dispatcher.ReplayByDeliveryID(ctx, deliveryID)
}

func cmdGitHubReplay(args []string) error {
	deliveryID, err := parseReplayDeliveryID(args)
	if err != nil {
		return err
	}

	replayed, err := githubReplayByDeliveryID(context.Background(), deliveryID)
	if err != nil {
		if errors.Is(err, ghwebhook.ErrDLQEntryNotFound) {
			return fmt.Errorf("delivery id %q not found in dlq", deliveryID)
		}
		return err
	}

	if replayed {
		fmt.Printf("Replayed delivery: %s\n", deliveryID)
		return nil
	}

	fmt.Printf("Delivery already replayed: %s\n", deliveryID)
	return nil
}

func parseReplayDeliveryID(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: ai-flow github replay --delivery-id <id>")
	}

	for i := 0; i < len(args); i++ {
		raw := strings.TrimSpace(args[i])
		if raw == "--delivery-id" {
			i++
			if i >= len(args) {
				return "", fmt.Errorf("usage: ai-flow github replay --delivery-id <id>")
			}
			value := strings.TrimSpace(args[i])
			if value == "" {
				return "", fmt.Errorf("usage: ai-flow github replay --delivery-id <id>")
			}
			return value, nil
		}
		if strings.HasPrefix(raw, "--delivery-id=") {
			value := strings.TrimSpace(strings.TrimPrefix(raw, "--delivery-id="))
			if value == "" {
				return "", fmt.Errorf("usage: ai-flow github replay --delivery-id <id>")
			}
			return value, nil
		}
	}

	return "", fmt.Errorf("usage: ai-flow github replay --delivery-id <id>")
}
