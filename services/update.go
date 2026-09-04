package services

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/vishnu-kyatannawar/nota/internal/config"
	"github.com/vishnu-kyatannawar/nota/internal/update"
)

// The phases an update passes through. The frontend renders from these alone,
// so a failure is a state to show rather than an error to throw.
const (
	UpdateIdle        = "idle"
	UpdateChecking    = "checking"
	UpdateCurrent     = "current"
	UpdateAvailable   = "available"
	UpdateDownloading = "downloading"
	UpdateReady       = "ready"
	UpdateFailed      = "failed"
)

// UpdateState is everything the frontend needs to render the update banner.
type UpdateState struct {
	Phase string `json:"phase"`
	// Version is the newer release, when there is one.
	Version string `json:"version"`
	// Percent is download progress, 0 unless downloading.
	Percent int `json:"percent"`
	// Message explains a failure; empty otherwise.
	Message string `json:"message"`
	// CanInstall is false when the binary is not ours to replace — a
	// package-managed or read-only install — and the UI offers the release
	// page instead.
	CanInstall bool `json:"canInstall"`
	// ReleaseURL is the page for the newer release.
	ReleaseURL string `json:"releaseUrl"`
}

// UpdateService checks for new releases and installs them.
type UpdateService struct {
	core *Core
}

// NewUpdateService returns the service bound as UpdateService.
func NewUpdateService(core *Core) *UpdateService { return &UpdateService{core: core} }

// SetUpdateEmitter wires state changes to the application event bus, so the
// frontend is told rather than having to poll. Unset in tests, where the
// returned state is the whole answer.
func (c *Core) SetUpdateEmitter(f func(UpdateState)) {
	c.updateMu.Lock()
	defer c.updateMu.Unlock()
	c.emitUpdate = f
}

func (c *Core) setUpdateState(s UpdateState) UpdateState {
	c.updateMu.Lock()
	c.updateState = s
	emit := c.emitUpdate
	c.updateMu.Unlock()
	if emit != nil {
		emit(s)
	}
	return s
}

// UpdateState returns the last known state.
func (c *Core) UpdateState() UpdateState {
	c.updateMu.Lock()
	defer c.updateMu.Unlock()
	return c.updateState
}

// State returns the current update state without checking anything.
func (u *UpdateService) State() UpdateState { return u.core.UpdateState() }

// Scheduled is the check the app runs on its own. It makes no request at all
// unless the user has agreed to automatic checks — this is the single gate on
// the only outbound traffic Nota has, which is why it lives here under test
// rather than inline in the scheduler.
func (u *UpdateService) Scheduled() UpdateState {
	if u.core.Settings().Updates.Check != config.UpdatesAuto {
		return u.core.UpdateState()
	}
	return u.Check(false)
}

// Check asks GitHub for the latest release.
//
// It never returns an error. A background check that fails goes quiet — an
// updater that complains every time the wifi is down is worse than none — while
// a check the user pressed reports what went wrong.
func (u *UpdateService) Check(manual bool) UpdateState {
	return u.check(context.Background(), manual)
}

func (u *UpdateService) check(ctx context.Context, manual bool) UpdateState {
	quiet := func(err error) UpdateState {
		if manual {
			return u.core.setUpdateState(UpdateState{Phase: UpdateFailed, Message: err.Error()})
		}
		return u.core.setUpdateState(UpdateState{Phase: UpdateIdle})
	}

	current, err := update.ParseVersion(u.core.version)
	if err != nil {
		// A build without a version — "dev" — has nothing to compare against.
		return quiet(fmt.Errorf("this build has no release version"))
	}

	u.core.setUpdateState(UpdateState{Phase: UpdateChecking})
	rel, err := u.core.updater.Latest(ctx)
	if err != nil {
		return quiet(err)
	}

	// Record the check even when nothing is new, so a relaunch does not ask again.
	_ = u.core.updateSettings(func(s *config.Settings) {
		s.Updates.LastCheck = time.Now().UTC().Format(time.RFC3339)
	})

	if !current.Less(rel.Version) {
		return u.core.setUpdateState(UpdateState{Phase: UpdateCurrent, Version: current.String()})
	}
	return u.core.setUpdateState(UpdateState{
		Phase:      UpdateAvailable,
		Version:    rel.Version.String(),
		CanInstall: installable(),
		ReleaseURL: rel.URL,
	})
}

// installable reports whether this process may replace its own binary.
func installable() bool {
	target, err := update.Target()
	if err != nil {
		return false
	}
	if !update.Supported() {
		return false
	}
	return update.CanReplace(target)
}

// Install downloads the newest release and puts it in place. The running
// process keeps working on the old binary until it is restarted.
func (u *UpdateService) Install() UpdateState {
	ctx := context.Background()
	fail := func(err error) UpdateState {
		return u.core.setUpdateState(UpdateState{Phase: UpdateFailed, Message: err.Error()})
	}

	target, err := update.Target()
	if err != nil {
		return fail(err)
	}
	if !update.CanReplace(target) {
		return fail(fmt.Errorf("%s is not writable by this user; install it the way you did the first time", target))
	}

	rel, err := u.core.updater.Latest(ctx)
	if err != nil {
		return fail(err)
	}
	current, err := update.ParseVersion(u.core.version)
	if err == nil && !current.Less(rel.Version) {
		return u.core.setUpdateState(UpdateState{Phase: UpdateCurrent, Version: current.String()})
	}

	u.core.setUpdateState(UpdateState{Phase: UpdateDownloading, Version: rel.Version.String()})
	archive, dir, err := u.core.updater.Fetch(ctx, rel, func(done, total int64) {
		if total <= 0 {
			return
		}
		u.core.setUpdateState(UpdateState{
			Phase:   UpdateDownloading,
			Version: rel.Version.String(),
			Percent: int(done * 100 / total),
		})
	})
	if err != nil {
		return fail(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	staged, err := update.Binary(archive, dir)
	if err != nil {
		return fail(err)
	}
	if err := update.Apply(staged, target); err != nil {
		return fail(err)
	}
	return u.core.setUpdateState(UpdateState{
		Phase:      UpdateReady,
		Version:    rel.Version.String(),
		ReleaseURL: rel.URL,
	})
}

// SetPreference records whether the app may check for updates. This is the only
// switch on any outbound network traffic the app makes.
func (u *UpdateService) SetPreference(check string) error {
	switch check {
	case config.UpdatesAsk, config.UpdatesAuto, config.UpdatesNever:
	default:
		return fmt.Errorf("unknown update preference %q", check)
	}
	return u.core.updateSettings(func(s *config.Settings) { s.Updates.Check = check })
}
