// Package repository holds mountabo's concrete persistence. ServerFile stores
// servers as a JSON file under the mountabo data dir; it satisfies the
// usecase.ServerStore port. SQLite can replace it later behind the same port.
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

// ServerFile persists servers to a single JSON file. All access is guarded by a
// mutex; reads and writes load/store the whole (small) list.
type ServerFile struct {
	path string
	mu   sync.Mutex
}

var _ usecase.ServerStore = (*ServerFile)(nil)

// NewServerFile returns a store backed by the given file path.
func NewServerFile(path string) *ServerFile {
	return &ServerFile{path: path}
}

func (f *ServerFile) load() ([]usecase.Server, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return []usecase.Server{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.path, err)
	}
	var servers []usecase.Server
	if len(data) == 0 {
		return []usecase.Server{}, nil
	}
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("parse %s: %w", f.path, err)
	}
	return servers, nil
}

func (f *ServerFile) persist(servers []usecase.Server) error {
	if err := os.MkdirAll(filepath.Dir(f.path), 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return fmt.Errorf("encode servers: %w", err)
	}
	// Write atomically: a temp file in the same dir, then rename.
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		return fmt.Errorf("replace %s: %w", f.path, err)
	}
	return nil
}

// List returns all stored servers.
func (f *ServerFile) List() ([]usecase.Server, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.load()
}

// Get returns one server by id, or usecase.ErrServerNotFound.
func (f *ServerFile) Get(id string) (usecase.Server, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	servers, err := f.load()
	if err != nil {
		return usecase.Server{}, err
	}
	for _, s := range servers {
		if s.ID == id {
			return s, nil
		}
	}
	return usecase.Server{}, usecase.ErrServerNotFound
}

// Save upserts a server by id.
func (f *ServerFile) Save(server usecase.Server) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	servers, err := f.load()
	if err != nil {
		return err
	}
	replaced := false
	for i, s := range servers {
		if s.ID == server.ID {
			servers[i] = server
			replaced = true
			break
		}
	}
	if !replaced {
		servers = append(servers, server)
	}
	return f.persist(servers)
}

// Delete removes a server by id. Deleting a missing server is not an error.
func (f *ServerFile) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	servers, err := f.load()
	if err != nil {
		return err
	}
	kept := servers[:0]
	for _, s := range servers {
		if s.ID != id {
			kept = append(kept, s)
		}
	}
	return f.persist(kept)
}
