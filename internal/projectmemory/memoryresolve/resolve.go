package memoryresolve

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func Resolve(
	request ResolutionRequest,
	index ResolutionIndex,
) (EntityResolutionResult, error) {
	if !index.Valid() {
		return nil, fmt.Errorf("resolution index is invalid")
	}
	if request.SnapshotBasis() != index.SnapshotBasis() {
		return newResolutionRetryRequired(
			request,
			index.SnapshotBasis(),
		)
	}
	scoped := scopeResolutionUnits(request.Context(), index.Units())
	exact := exactResolutionMatches(request.Query(), scoped)
	if len(exact) == 1 {
		return newExactEntity(
			request,
			exact[0].unit,
			exact[0].witnesses,
		)
	}
	if len(exact) > 1 {
		return resolveExactAmbiguity(request, index, exact)
	}
	lexical := lexicalResolutionMatches(request.Query(), scoped)
	if len(lexical) > 0 {
		return buildEntityCandidates(request, index, lexical)
	}
	if resolutionScopeComplete(request.Context(), index.Completeness()) {
		return newKnownAbsent(request, index)
	}
	issue, err := newIncompleteResolutionIndexIssue(index.Ref())
	if err != nil {
		return nil, err
	}
	return newResolutionUnsettled(
		request,
		index.SnapshotBasis(),
		[]ResolutionBasisIssue{issue},
	)
}

type resolutionMatch struct {
	unit       ResolutionUnit
	witnesses  []ResolutionWitness
	exactAlias *typedmemory.EntityAlias
}

func exactResolutionMatches(
	query ResolutionQuery,
	units []ResolutionUnit,
) []resolutionMatch {
	result := make([]resolutionMatch, 0)
	for _, unit := range units {
		if query.Original() == unit.Entity().ReferenceID().String() ||
			query.Original() ==
				unit.Entity().RefKind().String()+"/reference/"+
					unit.Entity().ReferenceID().String() {
			result = append(result, resolutionMatch{
				unit: unit,
				witnesses: []ResolutionWitness{
					newResolutionWitness(
						WitnessExactIdentifier,
						query.Original(),
						unit.Basis(),
					),
				},
			})
			continue
		}
		for _, alias := range unit.Aliases() {
			if query.Original() != alias.String() {
				continue
			}
			aliasValue := alias
			result = append(result, resolutionMatch{
				unit: unit,
				witnesses: []ResolutionWitness{
					newResolutionWitness(
						WitnessExactAlias,
						alias.String(),
						unit.Basis(),
					),
				},
				exactAlias: &aliasValue,
			})
		}
	}
	return result
}

func resolveExactAmbiguity(
	request ResolutionRequest,
	index ResolutionIndex,
	matches []resolutionMatch,
) (EntityResolutionResult, error) {
	aliasMatches := make([]resolutionMatch, 0)
	for _, match := range matches {
		if match.exactAlias == nil {
			continue
		}
		aliasMatches = append(aliasMatches, match)
	}
	if len(aliasMatches) > 0 {
		refs := make([]typedmemory.PersistedRef, 0, len(matches))
		for _, match := range matches {
			refs = append(refs, match.unit.Entity())
		}
		refs = canonicalCandidateRefs(refs)
		if len(refs) > 1 {
			issue := newAliasConflictIssue(
				*aliasMatches[0].exactAlias,
				refs,
			)
			return newResolutionUnsettled(
				request,
				index.SnapshotBasis(),
				[]ResolutionBasisIssue{issue},
			)
		}
	}
	if request.Context().Kind() == QueryAnyContext {
		candidates := make([]resolutionMatch, 0, len(matches))
		candidates = append(candidates, matches...)
		return buildEntityCandidates(request, index, candidates)
	}
	issue := newContextNotResolvedIssue(request.Query().Original())
	return newResolutionUnsettled(
		request,
		index.SnapshotBasis(),
		[]ResolutionBasisIssue{issue},
	)
}

func lexicalResolutionMatches(
	query ResolutionQuery,
	units []ResolutionUnit,
) []resolutionMatch {
	result := make([]resolutionMatch, 0)
	for _, unit := range units {
		witnesses := make([]ResolutionWitness, 0)
		labelTerms := termSet(unit.Label().String())
		labelMatches := intersectTerms(query.Terms(), labelTerms)
		if len(labelMatches) > 0 {
			witnesses = append(witnesses, newResolutionWitness(
				WitnessLexicalLabel,
				strings.Join(labelMatches, " "),
				unit.Basis(),
			))
		}
		for _, alias := range unit.Aliases() {
			aliasTerms := termSet(alias.String())
			aliasMatches := intersectTerms(query.Terms(), aliasTerms)
			if len(aliasMatches) == 0 {
				continue
			}
			witnesses = append(witnesses, newResolutionWitness(
				WitnessLexicalAlias,
				strings.Join(aliasMatches, " "),
				unit.Basis(),
			))
		}
		if len(witnesses) == 0 {
			continue
		}
		result = append(result, resolutionMatch{
			unit:      unit,
			witnesses: witnesses,
		})
	}
	sort.Slice(result, func(left int, right int) bool {
		leftStrength := resolutionMatchStrength(result[left])
		rightStrength := resolutionMatchStrength(result[right])
		if leftStrength != rightStrength {
			return leftStrength > rightStrength
		}
		return result[left].unit.key() < result[right].unit.key()
	})
	return result
}

func buildEntityCandidates(
	request ResolutionRequest,
	index ResolutionIndex,
	matches []resolutionMatch,
) (EntityCandidates, error) {
	matchCount, fits := resolutionCount(len(matches))
	if !fits {
		return EntityCandidates{}, fmt.Errorf(
			"entity-candidates result is invalid",
		)
	}
	included := matches
	if matchCount > uint64(request.MaxCandidates()) {
		// request.MaxCandidates() is smaller than len(matches) in this branch,
		// so it is representable as an int on the current architecture.
		limit := int(request.MaxCandidates()) // #nosec G115 -- bounded by len(matches) above.
		included = included[:limit]
	}
	includedCount, fits := resolutionCandidateCount(len(included))
	if !fits {
		return EntityCandidates{}, fmt.Errorf(
			"entity-candidates result is invalid",
		)
	}
	candidates := make([]EntityCandidate, 0, len(included))
	for position, match := range included {
		rank, rankFits := resolutionCandidateCount(position + 1)
		if !rankFits {
			return EntityCandidates{}, fmt.Errorf(
				"entity-candidates result is invalid",
			)
		}
		candidates = append(candidates, EntityCandidate{
			entity:    match.unit,
			rank:      rank,
			witnesses: append([]ResolutionWitness{}, match.witnesses...),
		})
	}
	inspected, fits := resolutionCount(len(scopeResolutionUnits(
		request.Context(),
		index.Units(),
	)))
	if !fits {
		return EntityCandidates{}, fmt.Errorf(
			"entity-candidates result is invalid",
		)
	}
	omitted := matchCount - uint64(includedCount)
	coverage := CandidateSetCoverage{
		index:          index.Ref(),
		indexVersion:   index.Version(),
		inspected:      inspected,
		included:       includedCount,
		omittedAtLeast: omitted,
	}
	if omitted > 0 {
		cursor, err := newResolutionCursor(
			index,
			index.SnapshotBasis(),
			request.Scope(),
			uint64(includedCount),
		)
		if err != nil {
			return EntityCandidates{}, err
		}
		coverage.cursor = cursor
	}
	return newEntityCandidates(request, index, candidates, coverage)
}

func scopeResolutionUnits(
	context QueryContext,
	units []ResolutionUnit,
) []ResolutionUnit {
	exact, exactContext := context.(ExactContext)
	if !exactContext {
		return append([]ResolutionUnit{}, units...)
	}
	result := make([]ResolutionUnit, 0)
	for _, unit := range units {
		if unit.Context() != exact.Context() {
			continue
		}
		result = append(result, unit)
	}
	return result
}

func resolutionScopeComplete(
	context QueryContext,
	completeness ResolutionIndexCompleteness,
) bool {
	if context.Kind() == QueryAnyContext {
		return completeness.CoversAllContexts()
	}
	exact, ok := context.(ExactContext)
	return ok && completeness.Covers(exact.Context())
}

func resolutionMatchStrength(match resolutionMatch) int {
	result := 0
	for _, witness := range match.witnesses {
		result += len(strings.Fields(witness.Matched()))
	}
	return result
}

func lexicalTerms(value string) []string {
	normalized := strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			return unicode.ToLower(character)
		}
		return ' '
	}, value)
	result := strings.Fields(normalized)
	sort.Strings(result)
	return slices.Compact(result)
}

func termSet(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, term := range lexicalTerms(value) {
		result[term] = struct{}{}
	}
	return result
}

func intersectTerms(
	query []string,
	candidate map[string]struct{},
) []string {
	result := make([]string, 0)
	for _, term := range query {
		if _, found := candidate[term]; !found {
			continue
		}
		result = append(result, term)
	}
	return result
}

func digestCanonical(value any) (typedmemory.SHA256Digest, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf(
			"encode memory resolution canonical value: %w",
			err,
		)
	}
	sum := sha256.Sum256(canonical)
	raw := "sha256:" + hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest(raw)
}
