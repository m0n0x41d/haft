package scopedrecall

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"

	"github.com/m0n0x41d/haft/internal/projectmemory/neighborhood"
)

type ProducerRef struct{ value string }
type ProducerVersion struct{ value string }

func NewProducerRef(raw string) (ProducerRef, error) {
	value, err := exactOneLine("candidate producer", raw)
	if err != nil {
		return ProducerRef{}, err
	}
	return ProducerRef{value: value}, nil
}

func NewProducerVersion(raw string) (ProducerVersion, error) {
	value, err := exactOneLine("candidate producer version", raw)
	if err != nil {
		return ProducerVersion{}, err
	}
	return ProducerVersion{value: value}, nil
}

func (ref ProducerRef) String() string         { return ref.value }
func (version ProducerVersion) String() string { return version.value }

type RecallQuery struct {
	original string
	terms    []string
	digest   string
}

func NewRecallQuery(raw string) (RecallQuery, error) {
	original := strings.TrimSpace(raw)
	if original == "" || original != raw {
		return RecallQuery{}, fmt.Errorf(
			"scoped recall query must be exact and non-empty",
		)
	}
	terms := lexicalTerms(original)
	if len(terms) == 0 {
		return RecallQuery{}, fmt.Errorf(
			"scoped recall query has no searchable terms",
		)
	}
	digestValue, err := digestCanonical(map[string]any{
		"original": original,
		"terms":    terms,
	})
	if err != nil {
		return RecallQuery{}, err
	}
	return RecallQuery{
		original: original,
		terms:    terms,
		digest:   digestValue.String(),
	}, nil
}

func (query RecallQuery) Original() string {
	return query.original
}

func (query RecallQuery) Terms() []string {
	return append([]string{}, query.terms...)
}

func (query RecallQuery) Digest() string {
	return query.digest
}

type CandidateBudget struct {
	maxCandidates uint32
}

func NewCandidateBudget(maxCandidates uint32) (CandidateBudget, error) {
	if maxCandidates == 0 {
		return CandidateBudget{}, fmt.Errorf(
			"candidate budget must be positive",
		)
	}
	return CandidateBudget{maxCandidates: maxCandidates}, nil
}

func (budget CandidateBudget) MaxCandidates() uint32 {
	return budget.maxCandidates
}

type ScopedRecallRequest struct {
	scope    ExactRecallScope
	snapshot neighborhood.SnapshotBasis
	query    RecallQuery
	budget   CandidateBudget
}

func NewScopedRecallRequest(
	scope ExactRecallScope,
	snapshot neighborhood.SnapshotBasis,
	query RecallQuery,
	budget CandidateBudget,
) (ScopedRecallRequest, error) {
	request := ScopedRecallRequest{
		scope:    scope,
		snapshot: snapshot,
		query:    query,
		budget:   budget,
	}
	if !scope.Valid() ||
		!snapshot.Valid() ||
		scope.Entity().RefKind().TypeEnv() != snapshot.TypeEnv() ||
		query.Original() == "" ||
		budget.MaxCandidates() == 0 {
		return ScopedRecallRequest{}, fmt.Errorf(
			"scoped recall request is invalid",
		)
	}
	return request, nil
}

func (request ScopedRecallRequest) Scope() ExactRecallScope {
	return request.scope
}

func (request ScopedRecallRequest) SnapshotBasis() neighborhood.SnapshotBasis {
	return request.snapshot
}

func (request ScopedRecallRequest) Query() RecallQuery {
	return request.query
}

func (request ScopedRecallRequest) Budget() CandidateBudget {
	return request.budget
}

type LexicalProducer struct {
	ref     ProducerRef
	version ProducerVersion
}

func NewLexicalProducer() LexicalProducer {
	ref, refErr := NewProducerRef("haft.lexical-recall")
	version, versionErr := NewProducerVersion("v1")
	if refErr != nil || versionErr != nil {
		panic("static lexical producer identity is invalid")
	}
	return LexicalProducer{
		ref:     ref,
		version: version,
	}
}

func (producer LexicalProducer) Ref() ProducerRef {
	return producer.ref
}

func (producer LexicalProducer) Version() ProducerVersion {
	return producer.version
}

type RecallCandidate struct {
	unit         RecallUnit
	producer     ProducerRef
	version      ProducerVersion
	rank         uint32
	matchedTerms []string
	exactPhrase  bool
}

func (candidate RecallCandidate) Unit() RecallUnit {
	return candidate.unit
}

func (candidate RecallCandidate) ProducerRef() ProducerRef {
	return candidate.producer
}

func (candidate RecallCandidate) ProducerVersion() ProducerVersion {
	return candidate.version
}

func (candidate RecallCandidate) Rank() uint32 {
	return candidate.rank
}

func (candidate RecallCandidate) MatchedTerms() []string {
	return append([]string{}, candidate.matchedTerms...)
}

func (candidate RecallCandidate) ExactPhraseMatched() bool {
	return candidate.exactPhrase
}

type lexicalCandidate struct {
	unit         RecallUnit
	matchedTerms []string
	exactPhrase  bool
}

func (producer LexicalProducer) Search(
	request ScopedRecallRequest,
	corpus ScopedCorpus,
) (ScopedRecallResult, error) {
	if corpus.Scope() != request.Scope() ||
		corpus.SnapshotBasis() != request.SnapshotBasis() {
		return nil, fmt.Errorf(
			"lexical producer received a differently scoped corpus",
		)
	}
	matches := lexicalMatches(request.Query(), corpus.Units())
	coverage, candidates, err := producer.applyBudget(
		request,
		uint64(len(corpus.Units())),
		matches,
	)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		basis, basisErr := NewNoMatchingMemoryBasis(
			[]ProducerRef{producer.Ref()},
		)
		if basisErr != nil {
			return nil, basisErr
		}
		return newScopedRecallAbstained(
			request,
			[]ProducerRef{producer.Ref()},
			basis,
		)
	}
	return newScopedMemoryCandidateSet(
		request,
		candidates,
		[]ProducerCoverage{coverage},
	)
}

func (producer LexicalProducer) applyBudget(
	request ScopedRecallRequest,
	inspectedCount uint64,
	matches []lexicalCandidate,
) (ProducerCoverage, []RecallCandidate, error) {
	limit := int(request.Budget().MaxCandidates())
	included := matches
	if len(included) > limit {
		included = included[:limit]
	}
	candidates := make([]RecallCandidate, 0, len(included))
	for index, match := range included {
		candidates = append(candidates, RecallCandidate{
			unit:         match.unit,
			producer:     producer.Ref(),
			version:      producer.Version(),
			rank:         uint32(index + 1),
			matchedTerms: append([]string{}, match.matchedTerms...),
			exactPhrase:  match.exactPhrase,
		})
	}
	if len(matches) <= limit {
		coverage, err := NewCompleteProducerCoverage(
			producer.Ref(),
			inspectedCount,
		)
		return coverage, candidates, err
	}
	cursor, err := NewRecallCursor(
		request.Scope(),
		request.SnapshotBasis(),
		producer.Ref(),
		request.Query(),
		uint64(len(candidates)),
	)
	if err != nil {
		return nil, nil, err
	}
	omitted, omittedOK := checkedSuffixCount(matches, len(candidates))
	if !omittedOK {
		return nil, nil, fmt.Errorf(
			"projected recall candidates exceed lexical matches",
		)
	}
	coverage, err := NewPartialProducerCoverage(
		producer.Ref(),
		inspectedCount,
		omitted,
		cursor,
	)
	return coverage, candidates, err
}

func checkedSuffixCount[T any](
	values []T,
	prefixLength int,
) (uint64, bool) {
	if prefixLength < 0 || prefixLength > len(values) {
		return 0, false
	}
	return uint64(len(values[prefixLength:])), true
}

func lexicalMatches(
	query RecallQuery,
	units []RecallUnit,
) []lexicalCandidate {
	result := make([]lexicalCandidate, 0)
	normalizedPhrase := normalizeLexicalText(query.Original())
	for _, unit := range units {
		normalizedText := normalizeLexicalText(unit.Text().String())
		textTerms := lexicalTermSet(normalizedText)
		matched := make([]string, 0)
		for _, term := range query.Terms() {
			if _, found := textTerms[term]; !found {
				continue
			}
			matched = append(matched, term)
		}
		if len(matched) == 0 &&
			!strings.Contains(normalizedText, normalizedPhrase) {
			continue
		}
		result = append(result, lexicalCandidate{
			unit:         unit,
			matchedTerms: matched,
			exactPhrase: strings.Contains(
				normalizedText,
				normalizedPhrase,
			),
		})
	}
	sort.Slice(result, func(left int, right int) bool {
		if result[left].exactPhrase != result[right].exactPhrase {
			return result[left].exactPhrase
		}
		if len(result[left].matchedTerms) != len(result[right].matchedTerms) {
			return len(result[left].matchedTerms) >
				len(result[right].matchedTerms)
		}
		return result[left].unit.ID().String() <
			result[right].unit.ID().String()
	})
	return result
}

func lexicalTerms(value string) []string {
	normalized := normalizeLexicalText(value)
	values := strings.Fields(normalized)
	sort.Strings(values)
	return slices.Compact(values)
}

func lexicalTermSet(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, term := range lexicalTerms(value) {
		result[term] = struct{}{}
	}
	return result
}

func normalizeLexicalText(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			return unicode.ToLower(character)
		}
		return ' '
	}, value)
}

func exactOneLine(label string, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || value != raw || strings.ContainsAny(value, "\r\n\t") {
		return "", fmt.Errorf("%s must be exact and one line", label)
	}
	return value, nil
}
