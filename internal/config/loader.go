package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func LoadFile(path string) (*Config, error) {
	return LoadGlobal(path)
}

func LoadGlobal(path string) (*Config, error) {
	cfg := Defaults()

	if path != "" {
		layer, err := loadLayerFromFile(path)
		if err != nil {
			return nil, err
		}
		ApplyConfigLayer(&cfg, layer)
	}

	if err := ApplyEnvOverrides(&cfg); err != nil {
		return nil, err
	}
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func LoadProject(repoPath string) (*ConfigLayer, error) {
	path := ProjectConfigPath(repoPath)
	layer, err := loadLayerFromFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &ConfigLayer{}, nil
		}
		return nil, err
	}
	return layer, nil
}

func loadLayerFromFile(path string) (*ConfigLayer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return loadLayerFromBytes(data)
}

func decodeLayerFromMap(raw map[string]any) (*ConfigLayer, error) {
	if raw == nil {
		return &ConfigLayer{}, nil
	}

	data, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal override map: %w", err)
	}
	return loadLayerFromBytes(data)
}

func loadLayerFromBytes(data []byte) (*ConfigLayer, error) {
	layer := &ConfigLayer{}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(layer); err != nil {
		return nil, err
	}
	return layer, nil
}
