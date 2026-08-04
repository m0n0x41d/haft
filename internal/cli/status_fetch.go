package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/codeintel"
)

const (
	statusRequestDeadline = 45 * time.Second
	statusCorpusDeadline  = 2 * time.Second
)

type statusDriftCorpusSource interface {
	PublishedDriftSymbolCorpus(
		context.Context,
		string,
	) (codeintel.DriftSymbolCorpusResult, error)
}

// fetchBoundedProjectStatusData keeps the public status attention surface
// bounded independently of the host timeout. Failure to establish a fresh
// symbol corpus degrades only the drift projection; the remaining status data
// is still returned and does not claim known absence.
func fetchBoundedProjectStatusData(
	ctx context.Context,
	store *artifact.Store,
	codeIntelService statusDriftCorpusSource,
	contextName string,
	projectRoot string,
) (artifact.StatusData, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	statusCtx, cancelStatus := context.WithTimeout(ctx, statusRequestDeadline)
	defer cancelStatus()

	var driftCorpus *artifact.DriftSymbolCorpus
	if codeIntelService == nil {
		driftCorpus = artifact.NewUnavailableDriftSymbolCorpus(
			"code-index service unavailable",
		)
	} else {
		corpusCtx, cancelCorpus := context.WithTimeout(statusCtx, statusCorpusDeadline)
		result, err := codeIntelService.PublishedDriftSymbolCorpus(
			corpusCtx,
			projectRoot,
		)
		cancelCorpus()
		switch {
		case err != nil:
			driftCorpus = artifact.NewUnavailableDriftSymbolCorpus(err.Error())
		case !result.Available:
			reason := strings.TrimSpace(result.Reason)
			if reason == "" {
				reason = fmt.Sprintf(
					"code-index freshness outcome %s did not establish a complete corpus",
					result.Refresh.Outcome,
				)
			}
			driftCorpus = artifact.NewUnavailableDriftSymbolCorpus(reason)
		case result.Current:
			driftCorpus = artifact.NewCompleteDriftSymbolCorpus(
				result.Corpus.Index.Epoch,
				result.Corpus.Index.Basis.BasisDigest,
				result.Corpus.Symbols,
			)
		default:
			driftCorpus = artifact.NewPartialDriftSymbolCorpus(
				result.Corpus.Index.Epoch,
				result.Corpus.Index.Basis.BasisDigest,
				result.Reason,
				result.Corpus.Symbols,
			)
		}
	}

	return artifact.FetchStatusDataWithOptions(
		statusCtx,
		store,
		contextName,
		artifact.StatusFetchOptions{
			ProjectRoot:       projectRoot,
			DriftSymbolCorpus: driftCorpus,
		},
	)
}
