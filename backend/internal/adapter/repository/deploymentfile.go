package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/goodylili/mountabo/internal/usecase"
)

// DeploymentFile persists configured deployments to a single JSON file, guarded
// by a mutex. It satisfies usecase.DeploymentStore; SQLite can replace it later
// behind the same port.
type DeploymentFile struct {
	path string
	mu   sync.Mutex
}

var _ usecase.DeploymentStore = (*DeploymentFile)(nil)

// NewDeploymentFile returns a store backed by the given file path.
func NewDeploymentFile(path string) *DeploymentFile {
	return &DeploymentFile{path: path}
}

func (f *DeploymentFile) load() ([]usecase.Deployment, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) || len(data) == 0 {
		return []usecase.Deployment{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.path, err)
	}
	var deployments []usecase.Deployment
	if err := json.Unmarshal(data, &deployments); err != nil {
		return nil, fmt.Errorf("parse %s: %w", f.path, err)
	}
	return deployments, nil
}

func (f *DeploymentFile) persist(deployments []usecase.Deployment) error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	data, err := json.MarshalIndent(deployments, "", "  ")
	if err != nil {
		return fmt.Errorf("encode deployments: %w", err)
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		return fmt.Errorf("replace %s: %w", f.path, err)
	}
	return nil
}

// List returns all configured deployments.
func (f *DeploymentFile) List() ([]usecase.Deployment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.load()
}

// Save upserts a deployment by owner+repo+branch (one record per deploy
// target), so re-deploying updates it rather than appending a duplicate.
func (f *DeploymentFile) Save(d usecase.Deployment) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	deployments, err := f.load()
	if err != nil {
		return err
	}
	replaced := false
	for i, existing := range deployments {
		if existing.Owner == d.Owner && existing.Repo == d.Repo && existing.Branch == d.Branch {
			d.ID = existing.ID // keep the original id on update
			deployments[i] = d
			replaced = true
			break
		}
	}
	if !replaced {
		deployments = append(deployments, d)
	}
	return f.persist(deployments)
}
