package cli

import (
	"errors"
	"fmt"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func loadProjectSpecificationSetSQLFirst(projectRoot string) (project.ProjectSpecificationSet, error) {
	sqlSpecSet, hasSQLSource, err := loadProjectSpecificationSetFromSQLEditions(projectRoot)
	if err != nil {
		return project.ProjectSpecificationSet{}, err
	}
	if hasSQLSource {
		return sqlSpecSet, nil
	}

	return project.LoadProjectSpecificationSet(projectRoot)
}

func loadProjectSpecificationSetFromSQLEditions(projectRoot string) (project.ProjectSpecificationSet, bool, error) {
	cfg, err := project.Load(haftDirFor(projectRoot))
	if err != nil {
		return project.ProjectSpecificationSet{}, false, err
	}
	if cfg == nil {
		return project.ProjectSpecificationSet{}, false, nil
	}

	projectID, store, closeStore, err := openSpecSectionEditionStore(projectRoot, cfg)
	if err != nil {
		return project.ProjectSpecificationSet{}, false, nil
	}
	defer closeStore()

	editions, err := store.ListCurrent(projectID)
	if err != nil {
		if errors.Is(err, specflow.ErrSpecSectionEditionNotFound) {
			return project.ProjectSpecificationSet{}, false, nil
		}
		return project.ProjectSpecificationSet{}, false, fmt.Errorf("read SQL spec section editions: %w", err)
	}
	if len(editions) == 0 {
		return project.ProjectSpecificationSet{}, false, nil
	}

	specSet, err := specflow.ProjectSpecificationSetFromEditions(editions)
	if err != nil {
		return project.ProjectSpecificationSet{}, true, fmt.Errorf("project specification SQL edition projection: %w", err)
	}

	return specSet, true, nil
}
