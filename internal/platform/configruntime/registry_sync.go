package configruntime

import (
	"context"
	"fmt"

	"github.com/yoke233/ai-workflow/internal/core"
)

type RegistrySyncStore interface {
	ListDrivers(ctx context.Context) ([]*core.AgentDriver, error)
	ListProfiles(ctx context.Context) ([]*core.AgentProfile, error)
	UpsertDriver(ctx context.Context, d *core.AgentDriver) error
	UpsertProfile(ctx context.Context, p *core.AgentProfile) error
	DeleteDriver(ctx context.Context, id string) error
	DeleteProfile(ctx context.Context, id string) error
}

func SyncRegistry(ctx context.Context, store RegistrySyncStore, snap *Snapshot) error {
	if store == nil || snap == nil {
		return nil
	}
	currentDrivers, err := store.ListDrivers(ctx)
	if err != nil {
		return fmt.Errorf("list drivers for sync: %w", err)
	}
	currentProfiles, err := store.ListProfiles(ctx)
	if err != nil {
		return fmt.Errorf("list profiles for sync: %w", err)
	}

	wantedDrivers := make(map[string]struct{}, len(snap.Drivers))
	for _, driver := range snap.Drivers {
		wantedDrivers[driver.ID] = struct{}{}
		if err := store.UpsertDriver(ctx, driver); err != nil {
			return fmt.Errorf("upsert driver %s: %w", driver.ID, err)
		}
	}

	wantedProfiles := make(map[string]struct{}, len(snap.Profiles))
	for _, profile := range snap.Profiles {
		wantedProfiles[profile.ID] = struct{}{}
		if err := store.UpsertProfile(ctx, profile); err != nil {
			return fmt.Errorf("upsert profile %s: %w", profile.ID, err)
		}
	}

	for _, profile := range currentProfiles {
		if _, ok := wantedProfiles[profile.ID]; ok {
			continue
		}
		if err := store.DeleteProfile(ctx, profile.ID); err != nil {
			return fmt.Errorf("delete stale profile %s: %w", profile.ID, err)
		}
	}

	for _, driver := range currentDrivers {
		if _, ok := wantedDrivers[driver.ID]; ok {
			continue
		}
		if err := store.DeleteDriver(ctx, driver.ID); err != nil {
			return fmt.Errorf("delete stale driver %s: %w", driver.ID, err)
		}
	}

	return nil
}
