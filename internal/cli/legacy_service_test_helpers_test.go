package cli

import (
	"context"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codeintel"
	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/recall"
)

// These adapters keep older tests concise while production always receives
// the request-scoped code-intelligence service explicitly.
func publishCodeExplore(
	ctx context.Context,
	store *artifact.Store,
	projectRoot string,
	symbol string,
	file string,
	line int,
	query string,
	maxCandidates int,
	view string,
	traceRef string,
) ([]byte, error) {
	return publishCodeExploreWithService(
		ctx,
		store,
		codeintel.NewService(store),
		projectRoot,
		symbol,
		file,
		line,
		query,
		maxCandidates,
		view,
		traceRef,
	)
}

func makeV5HandlerWithTaskMemoryProjection(
	store *artifact.Store,
	searcher recall.Searcher,
	crossHybrid *project.CrossHybrid,
	haftDir string,
	projectConfig *project.Config,
	indexStore *project.IndexStore,
	taskProjector taskMemoryProjector,
) fpf.V5ToolHandler {
	return makeV5HandlerWithTaskMemoryProjectionAndCodeIntel(
		store,
		searcher,
		crossHybrid,
		haftDir,
		projectConfig,
		indexStore,
		taskProjector,
		codeintel.NewService(store),
	)
}

func dispatchTool(
	ctx context.Context,
	store *artifact.Store,
	searcher recall.Searcher,
	haftDir string,
	name string,
	args map[string]any,
) (string, string, error) {
	return dispatchToolWithCodeIntel(
		ctx,
		store,
		searcher,
		haftDir,
		name,
		args,
		codeintel.NewService(store),
	)
}

func handleQuintRefresh(
	ctx context.Context,
	store *artifact.Store,
	haftDir string,
	args map[string]any,
) (string, error) {
	return handleQuintRefreshWithCodeIntel(
		ctx,
		store,
		haftDir,
		args,
		codeintel.NewService(store),
	)
}
