package persistence

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/GBoc09/SDCC_project/internal/registry"
)

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{
		path: path,
	}
}

func (s *FileStore) Save(
	state registry.RegistryState,
) error {
	directory := filepath.Dir(s.path)

	if err := os.MkdirAll(directory, 0o750); err != nil {
		return fmt.Errorf(
			"create state directory: %w",
			err,
		)
	}

	temporary, err := os.CreateTemp(
		directory,
		".registry-state-*.tmp",
	)
	if err != nil {
		return fmt.Errorf(
			"create temporary state file: %w",
			err,
		)
	}

	temporaryPath := temporary.Name()

	defer func() {
		_ = os.Remove(temporaryPath)
	}()

	encoder := json.NewEncoder(temporary)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(state); err != nil {
		_ = temporary.Close()

		return fmt.Errorf(
			"encode registry state: %w",
			err,
		)
	}

	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()

		return fmt.Errorf(
			"synchronize state file: %w",
			err,
		)
	}

	if err := temporary.Close(); err != nil {
		return fmt.Errorf(
			"close state file: %w",
			err,
		)
	}

	if err := os.Rename(
		temporaryPath,
		s.path,
	); err != nil {
		return fmt.Errorf(
			"replace state file: %w",
			err,
		)
	}

	return nil
}

func (s *FileStore) Load() (
	registry.RegistryState,
	error,
) {
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return registry.RegistryState{
			Services:  []registry.ServiceRecord{},
			Instances: []registry.InstanceRecord{},
		}, nil
	}
	if err != nil {
		return registry.RegistryState{},
			fmt.Errorf("open state file: %w", err)
	}
	defer file.Close()

	var state registry.RegistryState

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&state); err != nil {
		return registry.RegistryState{},
			fmt.Errorf("decode registry state: %w", err)
	}

	if err := ensureEndOfJSON(decoder); err != nil {
		return registry.RegistryState{},
			fmt.Errorf("decode registry state: %w", err)
	}

	return state, nil
}

func ensureEndOfJSON(decoder *json.Decoder) error {
	var extra any

	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}

	if err == nil {
		return errors.New("multiple JSON values")
	}

	return err
}
