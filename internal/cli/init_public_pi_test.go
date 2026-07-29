package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTypedPublicPiMigratesLegacySourceAndPreservesForeignState(
	t *testing.T,
) {
	projectRoot := physicalInitTestTempDir(t)
	homeRoot := physicalInitTestTempDir(t)
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	t.Setenv("HOME", homeRoot)

	settingsPath := filepath.Join(projectRoot, ".pi", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("create Pi settings root: %v", err)
	}
	if err := os.WriteFile(
		settingsPath,
		[]byte(`{"theme":"dark","packages":["npm:@foo/bar@1.0.0","./.haft/pi/haft-pi"]}`),
		0o640,
	); err != nil {
		t.Fatalf("write legacy Pi settings: %v", err)
	}
	if err := runTypedPublicPiForTest(t); err != nil {
		t.Fatalf("first typed Pi init: %v", err)
	}
	first, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read migrated Pi settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(first, &settings); err != nil {
		t.Fatalf("decode migrated Pi settings: %v", err)
	}
	wantPackages := []any{
		"npm:@foo/bar@1.0.0",
		"../.haft/pi/haft-pi",
	}
	if settings["theme"] != "dark" ||
		!reflect.DeepEqual(settings["packages"], wantPackages) {
		t.Fatalf("migrated Pi settings = %#v", settings)
	}
	info, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("stat migrated Pi settings: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("Pi settings mode = %o, want 640", info.Mode().Perm())
	}
	packageRoot := filepath.Join(
		projectRoot,
		filepath.FromSlash(piPackageRelDir),
	)
	if _, err := os.Stat(
		filepath.Join(packageRoot, "package.json"),
	); err != nil {
		t.Fatalf("typed Pi package missing: %v", err)
	}
	foreignPath := filepath.Join(packageRoot, "operator-note.txt")
	if err := os.WriteFile(
		foreignPath,
		[]byte("keep\n"),
		0o600,
	); err != nil {
		t.Fatalf("write foreign Pi package file: %v", err)
	}
	if err := runTypedPublicPiForTest(t); err != nil {
		t.Fatalf("second typed Pi init: %v", err)
	}
	second, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read second Pi settings: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf(
			"typed Pi init is not settings-idempotent\nfirst: %s\nsecond: %s",
			first,
			second,
		)
	}
	if content, err := os.ReadFile(foreignPath); err != nil ||
		string(content) != "keep\n" {
		t.Fatalf(
			"foreign Pi package file changed content=%q err=%v",
			content,
			err,
		)
	}
}

func TestTypedPublicPiOwnsObjectSourceWithoutOwningFilters(
	t *testing.T,
) {
	projectRoot := physicalInitTestTempDir(t)
	homeRoot := physicalInitTestTempDir(t)
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	t.Setenv("HOME", homeRoot)

	settingsPath := filepath.Join(projectRoot, ".pi", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("create Pi settings root: %v", err)
	}
	initial := []byte(
		"{\n" +
			"  \"packages\": [\n" +
			"    {\"source\":\"../.haft/pi/haft-pi\",\"skills\":[\"h-reason\"],\"prompts\":false}\n" +
			"  ],\n" +
			"  \"theme\": \"dark\"\n" +
			"}\n",
	)
	if err := os.WriteFile(settingsPath, initial, 0o640); err != nil {
		t.Fatalf("write object-form Pi settings: %v", err)
	}
	if err := runTypedPublicPiForTest(t); err != nil {
		t.Fatalf("adopt object-form Pi settings: %v", err)
	}
	adopted, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read adopted Pi settings: %v", err)
	}
	if !bytes.Equal(adopted, initial) {
		t.Fatalf(
			"object-form adoption rewrote user-owned fields:\n%s",
			adopted,
		)
	}

	changedFilters := []byte(
		"{\n" +
			"  \"packages\": [\n" +
			"    {\"source\":\"../.haft/pi/haft-pi\",\"skills\":[],\"prompts\":true}\n" +
			"  ],\n" +
			"  \"theme\": \"dark\"\n" +
			"}\n",
	)
	if err := os.WriteFile(
		settingsPath,
		changedFilters,
		0o640,
	); err != nil {
		t.Fatalf("change user-owned Pi filters: %v", err)
	}
	if err := runTypedPublicPiForTest(t); err != nil {
		t.Fatalf("rerun after Pi filter change: %v", err)
	}
	rerun, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read rerun Pi settings: %v", err)
	}
	if !bytes.Equal(rerun, changedFilters) {
		t.Fatalf("typed Pi init claimed user-owned filters:\n%s", rerun)
	}
}

func TestTypedPublicPiMigratesOnlyLegacyObjectSource(
	t *testing.T,
) {
	projectRoot := physicalInitTestTempDir(t)
	homeRoot := physicalInitTestTempDir(t)
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	t.Setenv("HOME", homeRoot)

	settingsPath := filepath.Join(projectRoot, ".pi", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("create Pi settings root: %v", err)
	}
	if err := os.WriteFile(
		settingsPath,
		[]byte(`{"packages":[{"source":"./.haft/pi/haft-pi","skills":["h-reason"],"prompts":false}],"theme":"dark"}`),
		0o644,
	); err != nil {
		t.Fatalf("write legacy object Pi settings: %v", err)
	}
	if err := runTypedPublicPiForTest(t); err != nil {
		t.Fatalf("migrate legacy object Pi settings: %v", err)
	}
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read migrated object Pi settings: %v", err)
	}
	var settings map[string]any
	if err := json.Unmarshal(content, &settings); err != nil {
		t.Fatalf("decode migrated object Pi settings: %v", err)
	}
	packages, ok := settings["packages"].([]any)
	if !ok || len(packages) != 1 {
		t.Fatalf("packages = %#v, want one object", settings["packages"])
	}
	object, ok := packages[0].(map[string]any)
	if !ok ||
		object["source"] != piSettingsEntry ||
		object["prompts"] != false ||
		!reflect.DeepEqual(object["skills"], []any{"h-reason"}) {
		t.Fatalf("migrated Pi package object = %#v", packages[0])
	}
}

func TestTypedPublicPiRejectsAmbiguousSourceBeforeWrites(
	t *testing.T,
) {
	projectRoot := physicalInitTestTempDir(t)
	homeRoot := physicalInitTestTempDir(t)
	restoreDirectory := changeInitTestDirectory(t, projectRoot)
	defer restoreDirectory()
	t.Setenv("HOME", homeRoot)

	settingsPath := filepath.Join(projectRoot, ".pi", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("create Pi settings root: %v", err)
	}
	fixture := []byte(
		`{"packages":["../.haft/pi/haft-pi",{"source":"./.haft/pi/haft-pi","skills":[]}]}`,
	)
	if err := os.WriteFile(settingsPath, fixture, 0o640); err != nil {
		t.Fatalf("write ambiguous Pi settings: %v", err)
	}
	err := runTypedPublicPiForTest(t)
	if err == nil || !strings.Contains(err.Error(), "is ambiguous") {
		t.Fatalf("ambiguous Pi settings error = %v", err)
	}
	after, readErr := os.ReadFile(settingsPath)
	if readErr != nil {
		t.Fatalf("read ambiguous Pi settings: %v", readErr)
	}
	if !bytes.Equal(after, fixture) {
		t.Fatalf("ambiguous Pi settings changed: %s", after)
	}
	if _, err := os.Stat(
		filepath.Join(projectRoot, ".haft", "config.yaml"),
	); !os.IsNotExist(err) {
		t.Fatalf("ambiguous Pi init wrote project core: %v", err)
	}
}

func runTypedPublicPiForTest(t *testing.T) error {
	t.Helper()
	restoreFlags := captureInitHostFlagState()
	defer restoreFlags.apply()
	clearInitHostFlags()
	initPi = true
	cmd := newPublicInitTestCommand()
	var selected bool
	cmd.Flags().BoolVar(&selected, "pi", false, "")
	if err := cmd.Flags().Set("pi", "true"); err != nil {
		t.Fatalf("set Pi flag: %v", err)
	}
	return runPublicInit(cmd, nil)
}
