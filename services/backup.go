package services

import (
	"path/filepath"
	"time"

	"github.com/vishnu-kyatannawar/nota/internal/export"
)

// BackupService exports the vault to a bundle and restores it again.
type BackupService struct {
	core *Core
}

// NewBackupService returns the service bound as BackupService.
func NewBackupService(core *Core) *BackupService { return &BackupService{core: core} }

// DefaultBundleName is the filename suggested in the save dialog.
func (b *BackupService) DefaultBundleName() string {
	return export.BundleName(time.Now().Format("2006-01-02"))
}

// Export writes the whole vault to a zip at dir, returning the bundle path.
// The index is left out; it is rebuilt from the notes on the next launch.
func (b *BackupService) Export(dir string) (string, error) {
	b.core.mu.RLock()
	defer b.core.mu.RUnlock()

	dest := filepath.Join(dir, b.DefaultBundleName())
	if err := export.Export(b.core.vault, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// Restore extracts a bundle over the vault and rebuilds the index from what
// lands on disk, so the cache can never disagree with the restored notes.
func (b *BackupService) Restore(bundle string) error {
	b.core.mu.Lock()
	defer b.core.mu.Unlock()

	if err := export.Restore(bundle, b.core.vault); err != nil {
		return err
	}
	return b.core.index.Rebuild(b.core.vault)
}
