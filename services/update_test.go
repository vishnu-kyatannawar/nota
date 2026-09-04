package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vishnu-kyatannawar/nota/internal/config"
	"github.com/vishnu-kyatannawar/nota/internal/update"
)

// updateCore builds a core whose updater points at a local server, so no test
// here touches the network.
func updateCore(t *testing.T, version, tag string) (*UpdateService, *Core) {
	t.Helper()
	core, err := NewCore(version, testSettings(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = core.Close() })

	body := []byte("archive")
	sum := sha256.Sum256(body)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/releases/latest"):
			fmt.Fprintf(w, `{"tag_name":%q}`, tag)
		case strings.HasSuffix(req.URL.Path, "/checksums.txt"):
			name, _ := update.AssetName(update.Version{Major: 4, Minor: 9, Patch: 0})
			fmt.Fprintf(w, "%s  %s\n", hex.EncodeToString(sum[:]), name)
		default:
			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(srv.Close)

	core.updater = &update.Client{HTTP: srv.Client(), Repo: "o/n", API: srv.URL, Download: srv.URL, Agent: "nota/test"}
	return NewUpdateService(core), core
}

func TestCheckReportsANewerRelease(t *testing.T) {
	u, _ := updateCore(t, "4.1.0", "v4.9.0")
	got := u.Check(true)
	if got.Phase != UpdateAvailable {
		t.Fatalf("phase = %q, want %q", got.Phase, UpdateAvailable)
	}
	if got.Version != "4.9.0" {
		t.Errorf("version = %q, want 4.9.0", got.Version)
	}
	if !strings.Contains(got.ReleaseURL, "v4.9.0") {
		t.Errorf("releaseUrl = %q, want the tag page", got.ReleaseURL)
	}
}

func TestCheckSaysNothingIsNewWhenTheReleaseMatches(t *testing.T) {
	u, _ := updateCore(t, "4.9.0", "v4.9.0")
	if got := u.Check(true); got.Phase != UpdateCurrent {
		t.Errorf("phase = %q, want %q", got.Phase, UpdateCurrent)
	}
}

func TestCheckNeverOffersADowngrade(t *testing.T) {
	u, _ := updateCore(t, "5.0.0", "v4.9.0")
	if got := u.Check(true); got.Phase != UpdateCurrent {
		t.Errorf("phase = %q, want %q — a newer local build must not be downgraded", got.Phase, UpdateCurrent)
	}
}

func TestADevBuildNeverReportsAnUpdate(t *testing.T) {
	// A locally built binary reports "dev", which is not a version.
	u, _ := updateCore(t, "dev", "v4.9.0")
	if got := u.Check(false); got.Phase != UpdateIdle {
		t.Errorf("background check on a dev build = %q, want %q", got.Phase, UpdateIdle)
	}
	if got := u.Check(true); got.Phase != UpdateFailed {
		t.Errorf("manual check on a dev build = %q, want %q", got.Phase, UpdateFailed)
	}
}

func TestABackgroundFailureStaysQuietAndAManualOneDoesNot(t *testing.T) {
	core, err := NewCore("4.1.0", testSettings(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = core.Close() }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden) // the unauthenticated rate limit
	}))
	defer srv.Close()
	core.updater = &update.Client{HTTP: srv.Client(), Repo: "o/n", API: srv.URL, Download: srv.URL, Agent: "nota/test"}
	u := NewUpdateService(core)

	if got := u.Check(false); got.Phase != UpdateIdle || got.Message != "" {
		t.Errorf("background failure = %+v, want a silent idle", got)
	}
	got := u.Check(true)
	if got.Phase != UpdateFailed {
		t.Fatalf("manual failure phase = %q, want %q", got.Phase, UpdateFailed)
	}
	if got.Message == "" {
		t.Error("a check the user pressed must say what went wrong")
	}
}

func TestCheckRecordsWhenItLastRan(t *testing.T) {
	u, core := updateCore(t, "4.1.0", "v4.9.0")
	if core.Settings().Updates.LastCheck != "" {
		t.Fatal("LastCheck is set before any check ran")
	}
	u.Check(true)
	if core.Settings().Updates.LastCheck == "" {
		t.Error("LastCheck was not recorded, so every relaunch would ask GitHub again")
	}
}

func TestEveryTransitionIsEmitted(t *testing.T) {
	u, core := updateCore(t, "4.1.0", "v4.9.0")
	var seen []string
	core.SetUpdateEmitter(func(s UpdateState) { seen = append(seen, s.Phase) })

	u.Check(true)
	want := []string{UpdateChecking, UpdateAvailable}
	if len(seen) != len(want) {
		t.Fatalf("emitted %v, want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("emitted %v, want %v", seen, want)
			break
		}
	}
}

func TestSetPreferenceOnlyAcceptsTheThreeChoices(t *testing.T) {
	u, core := updateCore(t, "4.1.0", "v4.9.0")
	if got := core.Settings().Updates.Check; got != config.UpdatesAsk {
		t.Fatalf("starting preference = %q, want %q", got, config.UpdatesAsk)
	}
	if err := u.SetPreference(config.UpdatesAuto); err != nil {
		t.Fatal(err)
	}
	if got := core.Settings().Updates.Check; got != config.UpdatesAuto {
		t.Errorf("preference = %q, want %q", got, config.UpdatesAuto)
	}
	if err := u.SetPreference("sure-why-not"); err == nil {
		t.Error("SetPreference accepted a value that is not a choice")
	}
	if got := core.Settings().Updates.Check; got != config.UpdatesAuto {
		t.Errorf("a rejected preference changed the setting to %q", got)
	}
}

func TestStateIsWhatTheLastTransitionLeft(t *testing.T) {
	u, _ := updateCore(t, "4.1.0", "v4.9.0")
	if got := u.State().Phase; got != UpdateIdle {
		t.Errorf("initial phase = %q, want %q", got, UpdateIdle)
	}
	u.Check(true)
	if got := u.State().Phase; got != UpdateAvailable {
		t.Errorf("phase after a check = %q, want %q", got, UpdateAvailable)
	}
}

func TestCheckHonoursACancelledContext(t *testing.T) {
	u, _ := updateCore(t, "4.1.0", "v4.9.0")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := u.check(ctx, true); got.Phase != UpdateFailed {
		t.Errorf("phase = %q, want %q for a cancelled check", got.Phase, UpdateFailed)
	}
}

// counted builds a core whose updater points at a server that counts every
// request, so a test can assert that none was made at all.
func counted(t *testing.T, version string, hits *int) (*UpdateService, *Core) {
	t.Helper()
	core, err := NewCore(version, testSettings(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = core.Close() })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*hits++
		fmt.Fprint(w, `{"tag_name":"v9.0.0"}`)
	}))
	t.Cleanup(srv.Close)
	core.updater = &update.Client{HTTP: srv.Client(), Repo: "o/n", API: srv.URL, Download: srv.URL, Agent: "nota/test"}
	return NewUpdateService(core), core
}

func TestNothingIsSentBeforeTheUserHasAnswered(t *testing.T) {
	// The whole privacy promise rests on this: a fresh install has made no
	// network request, and must not make one until the question is answered.
	hits := 0
	u, core := counted(t, "4.1.0", &hits)
	if got := core.Settings().Updates.Check; got != config.UpdatesAsk {
		t.Fatalf("a fresh install starts at %q, want %q", got, config.UpdatesAsk)
	}

	if got := u.Scheduled(); got.Phase != UpdateIdle {
		t.Errorf("phase = %q, want %q", got.Phase, UpdateIdle)
	}
	if hits != 0 {
		t.Errorf("%d request(s) were made before the user agreed, want 0", hits)
	}
}

func TestNothingIsSentAfterTheUserSaysNo(t *testing.T) {
	hits := 0
	u, _ := counted(t, "4.1.0", &hits)
	if err := u.SetPreference(config.UpdatesNever); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		u.Scheduled()
	}
	if hits != 0 {
		t.Errorf("%d request(s) were made after the user said no, want 0", hits)
	}
}

func TestTheScheduledCheckRunsOnceTheUserAgrees(t *testing.T) {
	hits := 0
	u, _ := counted(t, "4.1.0", &hits)
	if err := u.SetPreference(config.UpdatesAuto); err != nil {
		t.Fatal(err)
	}
	if got := u.Scheduled(); got.Phase != UpdateAvailable {
		t.Errorf("phase = %q, want %q", got.Phase, UpdateAvailable)
	}
	if hits != 1 {
		t.Errorf("made %d requests, want exactly 1", hits)
	}
}

func TestCheckNowWorksEvenWhenAutomaticChecksAreOff(t *testing.T) {
	// Pressing the button is itself the consent for that one request.
	hits := 0
	u, _ := counted(t, "4.1.0", &hits)
	if err := u.SetPreference(config.UpdatesNever); err != nil {
		t.Fatal(err)
	}
	if got := u.Check(true); got.Phase != UpdateAvailable {
		t.Errorf("phase = %q, want %q", got.Phase, UpdateAvailable)
	}
	if hits != 1 {
		t.Errorf("made %d requests, want exactly 1", hits)
	}
}
