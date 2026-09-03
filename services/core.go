// Package services holds the types bound into the frontend by Wails.
//
// Everything here is deliberately thin: a service validates its inputs, calls
// into internal/..., and returns. Keeping the logic out of this layer means the
// Wails v3 beta API can churn without dragging application behaviour with it,
// and it keeps internal/ testable without a running webview.
//
// The frontend never touches the filesystem. Every read and write goes through
// these services, so there is exactly one place a note can change — which is
// what makes external edits, the file watcher and restore safe to reason about.
package services

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/vishnu-kyatannawar/nota/internal/config"
	"github.com/vishnu-kyatannawar/nota/internal/index"
	"github.com/vishnu-kyatannawar/nota/internal/mdnote"
	"github.com/vishnu-kyatannawar/nota/internal/vault"
	"github.com/vishnu-kyatannawar/nota/internal/workplan"
)

// Core is the shared state every service works through.
type Core struct {
	mu       sync.RWMutex
	settings config.Settings
	vault    *vault.Vault
	index    *index.Index
	plans    *workplan.Manager
	version  string
}

// NewCore opens the vault and index described by settings.
func NewCore(version string, settings config.Settings) (*Core, error) {
	v, err := vault.Open(settings.VaultPath)
	if err != nil {
		return nil, err
	}
	idx, err := index.Open(settings.IndexPath())
	if err != nil {
		return nil, err
	}
	c := &Core{
		settings: settings,
		vault:    v,
		index:    idx,
		version:  version,
		plans: workplan.New(v, workplan.Options{
			Folder:           settings.WorkplanFolder,
			CreateOnWeekends: settings.CreateOnWeekends,
		}),
	}
	// The index is a cache; building it at startup costs little and guarantees
	// it reflects any edits made while the application was closed.
	if err := idx.Rebuild(v); err != nil {
		return nil, err
	}
	return c, nil
}

// Close releases the index.
func (c *Core) Close() error { return c.index.Close() }

// Vault exposes the open vault to the application wiring.
func (c *Core) Vault() *vault.Vault { return c.vault }

// Index exposes the open index to the application wiring.
func (c *Core) Index() *index.Index { return c.index }

// Plans exposes the workplan manager to the application wiring.
func (c *Core) Plans() *workplan.Manager { return c.plans }

// Settings returns a copy of the active settings.
func (c *Core) Settings() config.Settings {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.settings
}

// updateSettings applies a change to the settings and persists it under the
// core lock, so concurrent window events and UI toggles cannot interleave writes.
func (c *Core) updateSettings(apply func(*config.Settings)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	next := c.settings
	apply(&next)
	if err := config.Save(next.SettingsPath(), next); err != nil {
		return err
	}
	c.settings = next
	return nil
}

// newID mints a stable, sortable identifier for a note or an action item.
func newID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// saveNote writes a note and keeps the index in step with it.
func (c *Core) saveNote(path string, note *mdnote.Note) error {
	if err := c.vault.WriteNote(path, note); err != nil {
		return err
	}
	return c.index.Update(path, note)
}

// mutate applies a change to a note and saves it, which is the shape of nearly
// every editing operation the frontend performs.
func (c *Core) mutate(path string, apply func(*mdnote.Note) error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	note, err := c.vault.ReadNote(path)
	if err != nil {
		return err
	}
	if err := apply(note); err != nil {
		return err
	}
	return c.saveNote(path, note)
}
