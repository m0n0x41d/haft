//go:build darwin || linux

package codeintel

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codebase"

	_ "modernc.org/sqlite"
)

const indexHelperResultPrefix = "HAFT_INDEX_HELPER_RESULT="

type indexHelperResult struct {
	Outcome        IndexCoordinationOutcome `json:"outcome"`
	Epoch          int64                    `json:"epoch"`
	Reason         string                   `json:"reason,omitempty"`
	ProcessID      int                      `json:"process_id"`
	RefreshEntries int                      `json:"refresh_entries"`
}

func TestEnsureIndexMultiProcessSingleFlight(t *testing.T) {
	fixture := newIndexCoordinatorFixture(t, "qnt_f6f6f6f6")
	writeCoordinatorSource(t, fixture.root, "package sample\nfunc A() {}\n")
	events := canonicalTestDirectory(t)
	start := filepath.Join(events, "start")
	release := filepath.Join(events, "release-leader")
	leaderEntered := filepath.Join(events, "leader-entered")

	helpers := make([]*runningIndexHelper, 0, 4)
	helpers = append(helpers, startIndexHelper(t, fixture, indexHelperOptions{
		eventDirectory: events,
		startBarrier:   start,
		role:           "leader",
		pauseBefore:    release,
		leaderEntered:  leaderEntered,
	}))
	for range 3 {
		helpers = append(helpers, startIndexHelper(t, fixture, indexHelperOptions{
			eventDirectory: events,
			startBarrier:   start,
			role:           "follower",
			leaderEntered:  leaderEntered,
		}))
	}
	waitForIndexEvents(t, events, "ready-", 4)
	writeIndexEvent(t, start)
	waitForIndexEvents(t, events, "entered-", 1)
	waitForIndexEvents(t, events, "contended-", 3)
	if countIndexEvents(t, events, "entered-") != 1 ||
		countIndexEvents(t, events, "active-") != 1 {
		t.Fatalf("followers entered parser work while leader paused: %v", listIndexEvents(t, events))
	}
	writeIndexEvent(t, release)

	counts := map[IndexCoordinationOutcome]int{}
	for _, helper := range helpers {
		result := helper.wait(t)
		counts[result.Outcome]++
		if strings.Contains(strings.ToUpper(result.Reason), "SQLITE_BUSY") ||
			strings.Contains(strings.ToUpper(helper.stderr.String()), "SQLITE_BUSY") {
			t.Fatalf("SQLite publication contention: result=%#v stderr=%s", result, helper.stderr.String())
		}
	}
	if counts[IndexRebuiltPublished] != 1 || counts[IndexFreshAfterWait] != 3 {
		t.Fatalf("multi-process outcomes = %#v", counts)
	}
	if got := countIndexEvents(t, events, "entered-"); got != 1 {
		t.Fatalf("parser/build entries = %d, want 1", got)
	}
	if got := countIndexEvents(t, events, "published-"); got != 1 {
		t.Fatalf("publications = %d, want 1", got)
	}
	if got := countIndexEvents(t, events, "overlap-"); got != 0 {
		t.Fatalf("overlap observations = %d, want 0", got)
	}
	if got := countIndexEvents(t, events, "active-"); got != 0 {
		t.Fatalf("active refresh markers after completion = %d", got)
	}
}

func TestEnsureIndexOwnerCrashReleasesLeaseAndRetainsEpoch(t *testing.T) {
	fixture := newIndexCoordinatorFixture(t, "qnt_17171717")
	writeCoordinatorSource(t, fixture.root, "package sample\nfunc A() {}\n")
	service := NewServiceWithIndexCoordinator(fixture.store, fixture.coordinator)
	initial, err := service.EnsureIndex(context.Background(), fixture.root)
	if err != nil || !initial.Rebuilt() {
		t.Fatalf("initial = %#v, %v", initial, err)
	}
	writeCoordinatorSource(t, fixture.root, "package sample\nfunc A() {}\nfunc B() {}\n")
	events := canonicalTestDirectory(t)
	start := filepath.Join(events, "start-crash")
	release := filepath.Join(events, "never-release")
	crashing := startIndexHelper(t, fixture, indexHelperOptions{
		eventDirectory: events,
		startBarrier:   start,
		role:           "leader",
		pauseBefore:    release,
		leaderEntered:  filepath.Join(events, "leader-entered"),
	})
	waitForIndexEvents(t, events, "ready-", 1)
	writeIndexEvent(t, start)
	waitForIndexEvents(t, events, "entered-", 1)
	if err := crashing.command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = crashing.command.Wait()
	state, err := service.CurrentIndexState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state.Epoch != initial.PublishedEpoch {
		t.Fatalf("crashed owner changed epoch: got %d want %d", state.Epoch, initial.PublishedEpoch)
	}

	retryStart := filepath.Join(events, "start-retry")
	retry := startIndexHelper(t, fixture, indexHelperOptions{
		eventDirectory: events,
		startBarrier:   retryStart,
		role:           "retry",
	})
	waitForIndexEvents(t, events, "ready-", 2)
	writeIndexEvent(t, retryStart)
	retryResult := retry.wait(t)
	if retryResult.Outcome != IndexRebuiltPublished ||
		retryResult.Epoch != initial.PublishedEpoch+1 {
		t.Fatalf("retry after crash = %#v", retryResult)
	}
	if strings.Contains(strings.ToUpper(retryResult.Reason), "SQLITE_BUSY") {
		t.Fatalf("retry leaked SQLite contention: %#v", retryResult)
	}
}

func TestEnsureIndexSourceMutationWhileFollowerWaitsIsSequential(t *testing.T) {
	fixture := newIndexCoordinatorFixture(t, "qnt_28282828")
	writeCoordinatorSource(t, fixture.root, "package sample\nfunc A() {}\n")
	events := canonicalTestDirectory(t)
	leaderStart := filepath.Join(events, "start-leader")
	leaderRelease := filepath.Join(events, "release-after-publication")
	leader := startIndexHelper(t, fixture, indexHelperOptions{
		eventDirectory: events,
		startBarrier:   leaderStart,
		role:           "leader",
		pauseAfter:     leaderRelease,
	})
	waitForIndexEvents(t, events, "ready-", 1)
	writeIndexEvent(t, leaderStart)
	waitForIndexEvents(t, events, "after-refresh-paused-", 1)

	followerStart := filepath.Join(events, "start-follower")
	follower := startIndexHelper(t, fixture, indexHelperOptions{
		eventDirectory: events,
		startBarrier:   followerStart,
		role:           "follower",
	})
	waitForIndexEvents(t, events, "ready-", 2)
	writeIndexEvent(t, followerStart)
	waitForIndexEvents(t, events, "contended-", 1)
	writeCoordinatorSource(
		t,
		fixture.root,
		"package sample\nfunc A() {}\nfunc B() {}\n",
	)
	writeIndexEvent(t, leaderRelease)
	leaderResult := leader.wait(t)
	followerResult := follower.wait(t)
	if leaderResult.Outcome != IndexRebuiltPublished ||
		followerResult.Outcome != IndexRebuiltPublished {
		t.Fatalf("leader=%#v follower=%#v", leaderResult, followerResult)
	}
	if followerResult.Epoch != leaderResult.Epoch+1 {
		t.Fatalf("epochs are not sequential: leader=%d follower=%d", leaderResult.Epoch, followerResult.Epoch)
	}
	if countIndexEvents(t, events, "published-") != 2 ||
		countIndexEvents(t, events, "overlap-") != 0 {
		t.Fatalf("mutation events = %v", listIndexEvents(t, events))
	}
}

// TestIndexCoordinatorProcessHelper is a subprocess-only fixture. The parent
// test provides deterministic barrier and event carriers through the
// environment; ordinary package test runs return immediately.
func TestIndexCoordinatorProcessHelper(t *testing.T) {
	if os.Getenv("HAFT_INDEX_HELPER") != "1" {
		return
	}
	root := os.Getenv("HAFT_INDEX_ROOT")
	ledgerPath := os.Getenv("HAFT_INDEX_LEDGER")
	projectID := os.Getenv("HAFT_INDEX_PROJECT_ID")
	events := os.Getenv("HAFT_INDEX_EVENTS")
	start := os.Getenv("HAFT_INDEX_START")
	role := os.Getenv("HAFT_INDEX_ROLE")
	pid := os.Getpid()
	writeIndexHelperEvent(t, filepath.Join(events, fmt.Sprintf("ready-%d", pid)))
	waitForIndexHelperFile(t, start)
	if leaderEntered := os.Getenv("HAFT_INDEX_LEADER_ENTERED"); role == "follower" && leaderEntered != "" {
		waitForIndexHelperFile(t, leaderEntered)
	}

	database, err := sqlOpenIndexHelper(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	coordinator, err := NewProjectIndexCoordinator(ProjectIndexCoordinates{
		ProjectID:   projectID,
		ProjectRoot: root,
		LedgerPath:  ledgerPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	entries := 0
	coordinator.hooks.leaseBusy = func() {
		writeIndexHelperEvent(
			t,
			filepath.Join(events, fmt.Sprintf("contended-%d", pid)),
		)
	}
	coordinator.hooks.beforeRefresh = func(context.Context) error {
		entries++
		active := filepath.Join(events, fmt.Sprintf("active-%d", pid))
		writeIndexHelperEvent(t, active)
		if countIndexHelperEvents(t, events, "active-") > 1 {
			writeIndexHelperEvent(
				t,
				filepath.Join(events, fmt.Sprintf("overlap-%d", pid)),
			)
		}
		writeIndexHelperEvent(
			t,
			filepath.Join(events, fmt.Sprintf("entered-%d", pid)),
		)
		if role == "leader" {
			if leaderEntered := os.Getenv("HAFT_INDEX_LEADER_ENTERED"); leaderEntered != "" {
				writeIndexHelperEvent(t, leaderEntered)
			}
		}
		if pause := os.Getenv("HAFT_INDEX_PAUSE_BEFORE"); pause != "" {
			waitForIndexHelperFile(t, pause)
		}
		return nil
	}
	coordinator.hooks.afterRefresh = func(
		_ context.Context,
		refresh codebase.IndexRefreshResult,
		_ error,
	) error {
		if refresh.Published {
			writeIndexHelperEvent(
				t,
				filepath.Join(events, fmt.Sprintf("published-%d", pid)),
			)
		}
		if pause := os.Getenv("HAFT_INDEX_PAUSE_AFTER"); pause != "" {
			writeIndexHelperEvent(
				t,
				filepath.Join(events, fmt.Sprintf("after-refresh-paused-%d", pid)),
			)
			waitForIndexHelperFile(t, pause)
		}
		_ = os.Remove(filepath.Join(events, fmt.Sprintf("active-%d", pid)))
		return nil
	}
	service := NewServiceWithIndexCoordinator(
		artifact.NewStore(database),
		coordinator,
	)
	result, ensureErr := service.EnsureIndex(context.Background(), root)
	record := indexHelperResult{
		Outcome:        result.Outcome,
		Epoch:          result.PublishedEpoch,
		Reason:         coordinationFailureReason(errorsFromString(result.Reason), ensureErr),
		ProcessID:      pid,
		RefreshEntries: entries,
	}
	payload, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(os.Stdout, indexHelperResultPrefix+string(payload))
}

type indexHelperOptions struct {
	eventDirectory string
	startBarrier   string
	role           string
	pauseBefore    string
	pauseAfter     string
	leaderEntered  string
}

type runningIndexHelper struct {
	command *exec.Cmd
	stdout  bytes.Buffer
	stderr  bytes.Buffer
}

func startIndexHelper(
	t *testing.T,
	fixture *indexCoordinatorFixture,
	options indexHelperOptions,
) *runningIndexHelper {
	t.Helper()
	helper := &runningIndexHelper{}
	helper.command = exec.Command(
		os.Args[0],
		"-test.run=^TestIndexCoordinatorProcessHelper$",
		"-test.v",
	)
	helper.command.Env = append(os.Environ(),
		"HAFT_INDEX_HELPER=1",
		"HAFT_INDEX_ROOT="+fixture.root,
		"HAFT_INDEX_LEDGER="+fixture.ledgerPath,
		"HAFT_INDEX_PROJECT_ID="+fixture.coordinator.projectID,
		"HAFT_INDEX_EVENTS="+options.eventDirectory,
		"HAFT_INDEX_START="+options.startBarrier,
		"HAFT_INDEX_ROLE="+options.role,
		"HAFT_INDEX_PAUSE_BEFORE="+options.pauseBefore,
		"HAFT_INDEX_PAUSE_AFTER="+options.pauseAfter,
		"HAFT_INDEX_LEADER_ENTERED="+options.leaderEntered,
	)
	helper.command.Stdout = &helper.stdout
	helper.command.Stderr = &helper.stderr
	if err := helper.command.Start(); err != nil {
		t.Fatal(err)
	}
	return helper
}

func (helper *runningIndexHelper) wait(t *testing.T) indexHelperResult {
	t.Helper()
	if err := helper.command.Wait(); err != nil {
		t.Fatalf("index helper failed: %v\nstdout:\n%s\nstderr:\n%s", err, helper.stdout.String(), helper.stderr.String())
	}
	for _, line := range strings.Split(helper.stdout.String(), "\n") {
		if !strings.HasPrefix(line, indexHelperResultPrefix) {
			continue
		}
		var result indexHelperResult
		if err := json.Unmarshal(
			[]byte(strings.TrimPrefix(line, indexHelperResultPrefix)),
			&result,
		); err != nil {
			t.Fatal(err)
		}
		return result
	}
	t.Fatalf("index helper returned no result\nstdout:\n%s\nstderr:\n%s", helper.stdout.String(), helper.stderr.String())
	return indexHelperResult{}
}

func sqlOpenIndexHelper(path string) (*sql.DB, error) {
	return sql.Open(
		"sqlite",
		path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(3000)",
	)
}

func errorsFromString(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return fmt.Errorf("%s", value)
}

func writeIndexEvent(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("ready\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeIndexHelperEvent(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(
		path,
		[]byte(strconv.Itoa(os.Getpid())+"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
}

func waitForIndexEvents(
	t *testing.T,
	directory string,
	prefix string,
	want int,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if countIndexEvents(t, directory, prefix) >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d %q events: %v", want, prefix, listIndexEvents(t, directory))
}

func waitForIndexHelperFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for barrier %s", path)
}

func countIndexEvents(t *testing.T, directory string, prefix string) int {
	t.Helper()
	return countIndexHelperEvents(t, directory, prefix)
}

func countIndexHelperEvents(t *testing.T, directory string, prefix string) int {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			count++
		}
	}
	return count
}

func listIndexEvents(t *testing.T, directory string) []string {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
