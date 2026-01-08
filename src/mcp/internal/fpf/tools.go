package fpf

import (
	"path/filepath"
	"regexp"

	"github.com/m0n0x41d/quint-code/db"
	"github.com/m0n0x41d/quint-code/logger"
)

var slugifyRegex = regexp.MustCompile("[^a-zA-Z0-9]+")

type Tools struct {
	FSM     *FSM
	RootDir string
	DB      *db.Store
}

func NewTools(fsm *FSM, rootDir string, database *db.Store) *Tools {
	if database == nil {
		dbPath := filepath.Join(rootDir, ".quint", "quint.db")
		var err error
		database, err = db.NewStore(dbPath)
		if err != nil {
			logger.Warn().Err(err).Msg("failed to open database in NewTools")
		}
	}

	return &Tools{
		FSM:     fsm,
		RootDir: rootDir,
		DB:      database,
	}
}
