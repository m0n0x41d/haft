// Package kerneldbfixture provides isolated current-schema kernel stores for
// tests that exercise consumers of the database rather than schema migration.
package kerneldbfixture

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	kerneldb "github.com/m0n0x41d/haft/db"
)

var processCurrentSchemaTemplate currentSchemaTemplate

type currentSchemaTemplate struct {
	mu       sync.Mutex
	ready    bool
	contents []byte
	build    func() ([]byte, error)
}

// OpenCurrentStore opens databasePath as an isolated current-schema store.
// An absent database is cloned from one process-wide immutable template.
// Existing databases are opened in place so restart tests retain their state.
func OpenCurrentStore(databasePath string) (*kerneldb.Store, error) {
	exists, err := regularFileExists(databasePath)
	if err != nil {
		return nil, err
	}
	created := false
	if !exists {
		templateContents, err := processCurrentSchemaTemplate.load()
		if err != nil {
			return nil, err
		}
		err = cloneTemplate(templateContents, databasePath)
		if err != nil {
			return nil, err
		}
		created = true
	}
	store, err := kerneldb.NewStore(databasePath)
	if err != nil {
		openErr := fmt.Errorf("open current-schema kernel store: %w", err)
		if !created {
			return nil, openErr
		}
		return nil, errors.Join(
			openErr,
			removeCreatedStoreFiles(databasePath),
		)
	}
	return store, nil
}

func (template *currentSchemaTemplate) load() ([]byte, error) {
	template.mu.Lock()
	defer template.mu.Unlock()
	if template.ready {
		return template.contents, nil
	}
	build := template.build
	if build == nil {
		build = buildCurrentSchemaTemplate
	}
	contents, err := build()
	if err != nil {
		return nil, err
	}
	template.contents = contents
	template.ready = true
	return template.contents, nil
}

func buildCurrentSchemaTemplate() ([]byte, error) {
	return buildCurrentSchemaTemplateIn("")
}

func buildCurrentSchemaTemplateIn(
	parentDirectory string,
) (contents []byte, resultErr error) {
	directory, err := os.MkdirTemp(
		parentDirectory,
		"haft-kernel-schema-template-",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create kernel schema template directory: %w",
			err,
		)
	}
	defer func() {
		resultErr = joinTemplateCleanupError(
			resultErr,
			os.RemoveAll(directory),
		)
	}()

	databasePath := filepath.Join(directory, "current.db")
	store, err := kerneldb.NewStore(databasePath)
	if err != nil {
		return nil, fmt.Errorf(
			"build current kernel schema template: %w",
			err,
		)
	}
	database := store.GetRawDB()
	sealErr := sealTemplateDatabase(database)
	closeErr := store.Close()
	var wrappedCloseErr error
	if closeErr != nil {
		wrappedCloseErr = fmt.Errorf(
			"close current kernel schema template: %w",
			closeErr,
		)
	}
	if err := errors.Join(sealErr, wrappedCloseErr); err != nil {
		return nil, err
	}
	contents, err = os.ReadFile(databasePath)
	if err != nil {
		return nil, fmt.Errorf(
			"read current kernel schema template: %w",
			err,
		)
	}
	return contents, nil
}

func joinTemplateCleanupError(resultErr error, cleanupErr error) error {
	if cleanupErr == nil || resultErr == nil {
		return resultErr
	}
	return errors.Join(
		resultErr,
		fmt.Errorf("remove kernel schema template directory: %w", cleanupErr),
	)
}

func sealTemplateDatabase(database *sql.DB) error {
	var busy int
	var logFrames int
	var checkpointedFrames int
	err := database.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(
		&busy,
		&logFrames,
		&checkpointedFrames,
	)
	if err != nil {
		return fmt.Errorf("checkpoint current kernel schema template: %w", err)
	}
	if busy != 0 || logFrames != checkpointedFrames {
		return fmt.Errorf(
			"checkpoint current kernel schema template: busy=%d log=%d checkpointed=%d",
			busy,
			logFrames,
			checkpointedFrames,
		)
	}

	var journalMode string
	err = database.QueryRow("PRAGMA journal_mode=DELETE").Scan(&journalMode)
	if err != nil {
		return fmt.Errorf("seal current kernel schema template journal: %w", err)
	}
	if !strings.EqualFold(journalMode, "delete") {
		return fmt.Errorf(
			"seal current kernel schema template journal: got %q, want delete",
			journalMode,
		)
	}
	return nil
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect kernel store path: %w", err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("kernel store path is not a regular file")
	}
	return true, nil
}

func cloneTemplate(
	contents []byte,
	destinationPath string,
) (resultErr error) {
	destinationDirectory := filepath.Dir(destinationPath)
	err := os.MkdirAll(destinationDirectory, 0o755)
	if err != nil {
		return fmt.Errorf("create kernel store directory: %w", err)
	}
	temporary, err := os.CreateTemp(
		destinationDirectory,
		"."+filepath.Base(destinationPath)+".clone-*",
	)
	if err != nil {
		return fmt.Errorf("create temporary cloned kernel store: %w", err)
	}
	temporaryPath := temporary.Name()
	temporaryOpen := true
	defer func() {
		if temporaryOpen {
			if err := temporary.Close(); err != nil {
				resultErr = errors.Join(
					resultErr,
					fmt.Errorf(
						"close temporary cloned kernel store: %w",
						err,
					),
				)
			}
		}
		if err := os.Remove(temporaryPath); err != nil &&
			!errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(
				resultErr,
				fmt.Errorf(
					"remove temporary cloned kernel store: %w",
					err,
				),
			)
		}
	}()
	written, err := temporary.Write(contents)
	if err != nil {
		return fmt.Errorf("write current kernel schema template: %w", err)
	}
	if written != len(contents) {
		return fmt.Errorf(
			"write current kernel schema template: %w",
			io.ErrShortWrite,
		)
	}
	// The clone is a process-owned test fixture. Close establishes
	// read-after-write visibility; crash-durable fsync would add no test signal.
	err = temporary.Close()
	temporaryOpen = false
	if err != nil {
		return fmt.Errorf("close cloned kernel store: %w", err)
	}
	// Linking a complete sibling file publishes the destination atomically and
	// fails rather than replacing a path created by another fixture caller.
	if err := os.Link(temporaryPath, destinationPath); err != nil {
		return fmt.Errorf("publish cloned kernel store: %w", err)
	}
	return nil
}

func removeCreatedStoreFiles(databasePath string) error {
	var cleanupErr error
	for _, path := range []string{
		databasePath,
		databasePath + "-wal",
		databasePath + "-shm",
		databasePath + "-journal",
	} {
		if err := os.Remove(path); err != nil &&
			!errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(
				cleanupErr,
				fmt.Errorf(
					"remove failed current-schema kernel store %q: %w",
					path,
					err,
				),
			)
		}
	}
	return cleanupErr
}
