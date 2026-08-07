package codebase

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/textsearch"
)

const (
	MaxConcernQueryBytes          = 4096
	MaxDiscoveryCandidates        = 50
	discoveryProducerLimit        = 2000
	DiscoveryCandidates           = "candidate_set"
	DiscoveryNoCandidates         = "no_lexical_candidates"
	LexicalTierExactStableID      = "L0_exact_stable_id"
	LexicalTierExactQualifiedName = "L1_exact_qualified_name"
	LexicalTierExactName          = "L2_exact_name"
	LexicalTierAllTerms           = "L3_all_terms"
	LexicalTierPartialTerms       = "L4_partial_terms"
	LexicalTierEditDistance       = "L5_edit_distance"
	SymbolLaneProduction          = "production"
	SymbolLaneTest                = "test"
	SymbolLaneGenerated           = "generated"
	MatchFieldName                = "name"
	MatchFieldQualifiedName       = "qualified_name"
	MatchFieldReceiver            = "receiver"
	MatchFieldKind                = "kind"
	MatchFieldPath                = "file_path"
)

const symbolSearchFTSSchema = `CREATE VIRTUAL TABLE IF NOT EXISTS code_symbol_search USING fts5(
	symbol_id UNINDEXED,
	published_epoch UNINDEXED,
	name,
	qualified_name,
	receiver,
	kind,
	file_path,
	tokenize='unicode61'
)`

// ConcernQuery is one validated weak-to-strong query conversion. The original
// operator text remains available while all downstream search consumes the one
// parsed filter/term representation.
type ConcernQuery struct {
	raw    string
	parsed textsearch.ParsedQuery
	terms  []string
}

func NewConcernQuery(raw string) (ConcernQuery, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ConcernQuery{}, fmt.Errorf("concern query must not be blank")
	}
	if len(raw) > MaxConcernQueryBytes {
		return ConcernQuery{}, fmt.Errorf(
			"concern query exceeds %d bytes",
			MaxConcernQueryBytes,
		)
	}
	parsed := textsearch.ParseQuery(raw)
	termSource := strings.TrimSpace(
		strings.Join(
			append([]string{parsed.Text}, parsed.NameFilters...),
			" ",
		),
	)
	terms := textsearch.Terms(
		termSource,
		textsearch.Options{Stems: true},
	)
	if len(terms) == 0 {
		return ConcernQuery{}, fmt.Errorf(
			"concern query has no searchable lexical terms",
		)
	}
	return ConcernQuery{
		raw:    raw,
		parsed: parsed,
		terms:  append([]string{}, terms...),
	}, nil
}

func (q ConcernQuery) Raw() string {
	return q.raw
}

func (q ConcernQuery) Terms() []string {
	return append([]string{}, q.terms...)
}

func (q ConcernQuery) PathFilters() []string {
	return append([]string{}, q.parsed.PathFilters...)
}

func (q ConcernQuery) NameFilters() []string {
	return append([]string{}, q.parsed.NameFilters...)
}

func (q ConcernQuery) KindFilters() []string {
	return append([]string{}, q.parsed.Kinds...)
}

func (q ConcernQuery) LanguageFilters() []string {
	return append([]string{}, q.parsed.Langs...)
}

func (q ConcernQuery) valid() bool {
	rebuilt, err := NewConcernQuery(q.raw)
	return err == nil &&
		reflectParsedQuery(rebuilt.parsed) == reflectParsedQuery(q.parsed) &&
		strings.Join(rebuilt.terms, "\x00") ==
			strings.Join(q.terms, "\x00")
}

func (q ConcernQuery) exactText() string {
	return strings.TrimSpace(q.parsed.Text)
}

func (q ConcernQuery) MarshalJSON() ([]byte, error) {
	if !q.valid() {
		return nil, fmt.Errorf("marshal invalid concern query")
	}
	payload := struct {
		Raw       string   `json:"raw"`
		Terms     []string `json:"terms"`
		Kinds     []string `json:"kind_filters,omitempty"`
		Languages []string `json:"language_filters,omitempty"`
		Paths     []string `json:"path_filters,omitempty"`
		Names     []string `json:"name_filters,omitempty"`
	}{
		Raw:       q.raw,
		Terms:     q.Terms(),
		Kinds:     q.KindFilters(),
		Languages: q.LanguageFilters(),
		Paths:     q.PathFilters(),
		Names:     q.NameFilters(),
	}
	return json.Marshal(payload)
}

func reflectParsedQuery(query textsearch.ParsedQuery) string {
	parts := []string{
		query.Text,
		strings.Join(query.Kinds, "\x01"),
		strings.Join(query.Langs, "\x01"),
		strings.Join(query.PathFilters, "\x01"),
		strings.Join(query.NameFilters, "\x01"),
	}
	return strings.Join(parts, "\x00")
}

// DiscoveryBudget is the validated public candidate cap. The producer cap is
// fixed separately so a caller cannot turn a concern query into an unbounded
// symbol scan.
type DiscoveryBudget struct {
	maxCandidates int
}

func NewDiscoveryBudget(maxCandidates int) (DiscoveryBudget, error) {
	if maxCandidates < 1 ||
		maxCandidates > MaxDiscoveryCandidates {
		return DiscoveryBudget{}, fmt.Errorf(
			"discovery candidate limit must be between 1 and %d",
			MaxDiscoveryCandidates,
		)
	}
	return DiscoveryBudget{maxCandidates: maxCandidates}, nil
}

func (b DiscoveryBudget) MaxCandidates() int {
	return b.maxCandidates
}

func (b DiscoveryBudget) valid() bool {
	_, err := NewDiscoveryBudget(b.maxCandidates)
	return err == nil
}

// LexicalTier is precedence, not a probability or confidence value.
type LexicalTier struct {
	code string
}

func (t LexicalTier) String() string {
	return t.code
}

func lexicalTier(code string) LexicalTier {
	return LexicalTier{code: code}
}

type SymbolSourceLane struct {
	code string
}

func (l SymbolSourceLane) String() string {
	return l.code
}

type MatchField struct {
	code string
}

func (f MatchField) String() string {
	return f.code
}

// SymbolTermMatch makes the exact lexical support inspectable.
type SymbolTermMatch struct {
	term   string
	fields []MatchField
}

func (m SymbolTermMatch) Term() string {
	return m.term
}

func (m SymbolTermMatch) Fields() []MatchField {
	return append([]MatchField{}, m.fields...)
}

func (m SymbolTermMatch) MarshalJSON() ([]byte, error) {
	fields := make([]string, 0, len(m.fields))
	for _, field := range m.fields {
		fields = append(fields, field.String())
	}
	return json.Marshal(struct {
		Term   string   `json:"term"`
		Fields []string `json:"fields"`
	}{
		Term:   m.term,
		Fields: fields,
	})
}

// TermCoverage is an exact fraction rather than a rank-dependent float.
type TermCoverage struct {
	covered int
	total   int
}

func (c TermCoverage) Covered() int {
	return c.covered
}

func (c TermCoverage) Total() int {
	return c.total
}

func (c TermCoverage) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Covered int `json:"covered"`
		Total   int `json:"total"`
	}{
		Covered: c.covered,
		Total:   c.total,
	})
}

// SymbolDiscoveryCandidate contains an existing canonical symbol plus the
// complete deterministic evidence that placed it in this candidate set.
type SymbolDiscoveryCandidate struct {
	symbol        CodeSymbol
	tier          LexicalTier
	matches       []SymbolTermMatch
	coverage      TermCoverage
	fieldCoverage TermCoverage
	lane          SymbolSourceLane
	epoch         int64
	filterHits    int
	bestField     int
}

func (c SymbolDiscoveryCandidate) Symbol() CodeSymbol {
	return c.symbol
}

func (c SymbolDiscoveryCandidate) Tier() LexicalTier {
	return c.tier
}

func (c SymbolDiscoveryCandidate) Matches() []SymbolTermMatch {
	return append([]SymbolTermMatch{}, c.matches...)
}

func (c SymbolDiscoveryCandidate) Coverage() TermCoverage {
	return c.coverage
}

func (c SymbolDiscoveryCandidate) FieldCoverage() TermCoverage {
	return c.fieldCoverage
}

func (c SymbolDiscoveryCandidate) Lane() SymbolSourceLane {
	return c.lane
}

func (c SymbolDiscoveryCandidate) Epoch() int64 {
	return c.epoch
}

func (c SymbolDiscoveryCandidate) MarshalJSON() ([]byte, error) {
	if c.symbol.AnchorID == "" || c.epoch < 1 {
		return nil, fmt.Errorf("marshal invalid symbol discovery candidate")
	}
	return json.Marshal(struct {
		Symbol        CodeSymbol        `json:"symbol"`
		Tier          string            `json:"tier"`
		Matches       []SymbolTermMatch `json:"matches"`
		Coverage      TermCoverage      `json:"term_coverage"`
		FieldCoverage TermCoverage      `json:"matched_field_coverage"`
		Lane          string            `json:"lane"`
		Epoch         int64             `json:"epoch"`
	}{
		Symbol:        c.symbol,
		Tier:          c.tier.String(),
		Matches:       c.Matches(),
		Coverage:      c.coverage,
		FieldCoverage: c.fieldCoverage,
		Lane:          c.lane.String(),
		Epoch:         c.epoch,
	})
}

type SymbolDiscoveryKind struct {
	code string
}

func (k SymbolDiscoveryKind) String() string {
	return k.code
}

// SymbolDiscoveryBatch is a closed candidate-set/no-candidate outcome. It
// deliberately has no selected-symbol field.
type SymbolDiscoveryBatch struct {
	kind       SymbolDiscoveryKind
	query      ConcernQuery
	candidates []SymbolDiscoveryCandidate
	budget     DiscoveryBudget
	epoch      int64
}

func (b SymbolDiscoveryBatch) Kind() SymbolDiscoveryKind {
	return b.kind
}

func (b SymbolDiscoveryBatch) Query() ConcernQuery {
	return b.query
}

func (b SymbolDiscoveryBatch) Candidates() []SymbolDiscoveryCandidate {
	return append([]SymbolDiscoveryCandidate{}, b.candidates...)
}

func (b SymbolDiscoveryBatch) Budget() DiscoveryBudget {
	return b.budget
}

func (b SymbolDiscoveryBatch) Epoch() int64 {
	return b.epoch
}

func (b SymbolDiscoveryBatch) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind          string                     `json:"kind"`
		Query         ConcernQuery               `json:"query"`
		Candidates    []SymbolDiscoveryCandidate `json:"candidates"`
		MaxCandidates int                        `json:"max_candidates"`
		Epoch         int64                      `json:"epoch"`
	}{
		Kind:          b.kind.String(),
		Query:         b.query,
		Candidates:    b.Candidates(),
		MaxCandidates: b.budget.MaxCandidates(),
		Epoch:         b.epoch,
	})
}

// DiscoverSymbols resolves a validated concern to an evidence-bearing bounded
// candidate set under one exact published epoch.
func (s *SymbolStore) DiscoverSymbols(
	ctx context.Context,
	query ConcernQuery,
	budget DiscoveryBudget,
	epoch int64,
) (SymbolDiscoveryBatch, error) {
	if !query.valid() {
		return SymbolDiscoveryBatch{}, fmt.Errorf("invalid concern query")
	}
	if !budget.valid() {
		return SymbolDiscoveryBatch{}, fmt.Errorf("invalid discovery budget")
	}
	if epoch < 1 {
		return SymbolDiscoveryBatch{}, fmt.Errorf(
			"published discovery epoch must be positive",
		)
	}
	searchEpoch, err := s.SymbolSearchEpoch(ctx)
	if err != nil {
		return SymbolDiscoveryBatch{}, err
	}
	if searchEpoch != epoch {
		return SymbolDiscoveryBatch{}, fmt.Errorf(
			"search_epoch_mismatch: search epoch %d, requested epoch %d",
			searchEpoch,
			epoch,
		)
	}
	exact, err := s.discoverExactSymbols(ctx, query, epoch)
	if err != nil {
		return SymbolDiscoveryBatch{}, err
	}
	if len(exact) > 0 {
		return symbolDiscoveryBatch(query, budget, epoch, exact), nil
	}
	lexical, err := s.discoverLexicalSymbols(ctx, query, epoch)
	if err != nil {
		return SymbolDiscoveryBatch{}, err
	}
	if len(lexical) > 0 {
		return symbolDiscoveryBatch(query, budget, epoch, lexical), nil
	}
	typos, err := s.discoverEditDistanceSymbols(ctx, query, epoch)
	if err != nil {
		return SymbolDiscoveryBatch{}, err
	}
	return symbolDiscoveryBatch(query, budget, epoch, typos), nil
}

func symbolDiscoveryBatch(
	query ConcernQuery,
	budget DiscoveryBudget,
	epoch int64,
	candidates []SymbolDiscoveryCandidate,
) SymbolDiscoveryBatch {
	sortSymbolDiscoveryCandidates(candidates)
	if len(candidates) > budget.MaxCandidates() {
		candidates = candidates[:budget.MaxCandidates()]
	}
	kind := DiscoveryNoCandidates
	if len(candidates) > 0 {
		kind = DiscoveryCandidates
	}
	return SymbolDiscoveryBatch{
		kind:       SymbolDiscoveryKind{code: kind},
		query:      query,
		candidates: append([]SymbolDiscoveryCandidate{}, candidates...),
		budget:     budget,
		epoch:      epoch,
	}
}

func (s *SymbolStore) discoverExactSymbols(
	ctx context.Context,
	query ConcernQuery,
	epoch int64,
) ([]SymbolDiscoveryCandidate, error) {
	exactText := query.exactText()
	if exactText == "" {
		return nil, nil
	}
	stable, err := s.querySymbols(
		ctx,
		codeSymbolSelect+
			` WHERE id = ? OR anchor_id = ? ORDER BY anchor_id, file_path`,
		exactText,
		exactText,
	)
	if err != nil {
		return nil, err
	}
	stable = applySymbolFilters(stable, query.parsed)
	if len(stable) > 0 {
		return candidatesForExactTier(
			stable,
			query,
			epoch,
			LexicalTierExactStableID,
			MatchFieldQualifiedName,
		), nil
	}
	qualified, err := s.querySymbols(
		ctx,
		codeSymbolSelect+
			` WHERE qualified_name = ? ORDER BY anchor_id, file_path`,
		exactText,
	)
	if err != nil {
		return nil, err
	}
	qualified = applySymbolFilters(qualified, query.parsed)
	if len(qualified) > 0 {
		return candidatesForExactTier(
			qualified,
			query,
			epoch,
			LexicalTierExactQualifiedName,
			MatchFieldQualifiedName,
		), nil
	}
	name, err := s.querySymbols(
		ctx,
		codeSymbolSelect+
			` WHERE name = ? ORDER BY anchor_id, file_path`,
		exactText,
	)
	if err != nil {
		return nil, err
	}
	name = applySymbolFilters(name, query.parsed)
	return candidatesForExactTier(
		name,
		query,
		epoch,
		LexicalTierExactName,
		MatchFieldName,
	), nil
}

func candidatesForExactTier(
	symbols []CodeSymbol,
	query ConcernQuery,
	epoch int64,
	tier string,
	field string,
) []SymbolDiscoveryCandidate {
	candidates := make([]SymbolDiscoveryCandidate, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol.AnchorID == "" {
			continue
		}
		candidates = append(
			candidates,
			SymbolDiscoveryCandidate{
				symbol: symbol,
				tier:   lexicalTier(tier),
				matches: []SymbolTermMatch{
					{
						term: query.exactText(),
						fields: []MatchField{
							{code: field},
						},
					},
				},
				coverage:      TermCoverage{covered: 1, total: 1},
				fieldCoverage: TermCoverage{covered: 1, total: 1},
				lane:          classifySymbolSourceLane(symbol.FilePath),
				epoch:         epoch,
				filterHits:    symbolFilterHits(symbol, query.parsed),
				bestField:     matchFieldRank(field),
			},
		)
	}
	return candidates
}

func (s *SymbolStore) discoverLexicalSymbols(
	ctx context.Context,
	query ConcernQuery,
	epoch int64,
) ([]SymbolDiscoveryCandidate, error) {
	symbols, err := s.searchSymbolCandidates(
		ctx,
		symbolSearchExpression(query.Terms()),
		query.parsed,
		epoch,
		discoveryProducerLimit,
	)
	if err != nil {
		return nil, err
	}
	candidates := make([]SymbolDiscoveryCandidate, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol.AnchorID == "" {
			continue
		}
		if len(applySymbolFilters(
			[]CodeSymbol{symbol},
			query.parsed,
		)) == 0 {
			continue
		}
		matches, bestField, fieldCoverage := symbolTermMatches(
			symbol,
			query.Terms(),
		)
		if len(matches) == 0 {
			continue
		}
		tier := LexicalTierPartialTerms
		if len(matches) == len(query.Terms()) {
			tier = LexicalTierAllTerms
		}
		candidates = append(
			candidates,
			SymbolDiscoveryCandidate{
				symbol:  symbol,
				tier:    lexicalTier(tier),
				matches: matches,
				coverage: TermCoverage{
					covered: len(matches),
					total:   len(query.Terms()),
				},
				fieldCoverage: fieldCoverage,
				lane:          classifySymbolSourceLane(symbol.FilePath),
				epoch:         epoch,
				filterHits:    symbolFilterHits(symbol, query.parsed),
				bestField:     bestField,
			},
		)
	}
	return candidates, nil
}

func (s *SymbolStore) discoverEditDistanceSymbols(
	ctx context.Context,
	query ConcernQuery,
	epoch int64,
) ([]SymbolDiscoveryCandidate, error) {
	symbols, err := s.scanAllSymbols(ctx, editScanCap)
	if err != nil {
		return nil, err
	}
	symbols = applySymbolFilters(symbols, query.parsed)
	candidates := make([]SymbolDiscoveryCandidate, 0)
	for _, symbol := range symbols {
		if symbol.AnchorID == "" {
			continue
		}
		name := strings.ToLower(symbol.Name)
		for _, term := range query.Terms() {
			maxDistance := maxEditDist(term)
			distance := textsearch.BoundedEditDistance(
				name,
				strings.ToLower(term),
				maxDistance,
			)
			if distance > maxDistance {
				continue
			}
			candidates = append(
				candidates,
				SymbolDiscoveryCandidate{
					symbol: symbol,
					tier: lexicalTier(
						LexicalTierEditDistance,
					),
					matches: []SymbolTermMatch{
						{
							term: term,
							fields: []MatchField{
								{code: MatchFieldName},
							},
						},
					},
					coverage: TermCoverage{
						covered: 1,
						total:   len(query.Terms()),
					},
					fieldCoverage: TermCoverage{
						covered: 1,
						total:   1,
					},
					lane:       classifySymbolSourceLane(symbol.FilePath),
					epoch:      epoch,
					filterHits: symbolFilterHits(symbol, query.parsed),
					bestField:  matchFieldRank(MatchFieldName),
				},
			)
			break
		}
	}
	return candidates, nil
}

func (s *SymbolStore) querySymbols(
	ctx context.Context,
	statement string,
	args ...any,
) ([]CodeSymbol, error) {
	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCodeSymbols(rows)
}

func (s *SymbolStore) searchSymbolCandidates(
	ctx context.Context,
	expression string,
	filters textsearch.ParsedQuery,
	epoch int64,
	limit int,
) ([]CodeSymbol, error) {
	statement := `
		SELECT
		  code_symbols.id,
		  code_symbols.anchor_id,
		  code_symbols.anchor_version,
		  code_symbols.file_path,
		  code_symbols.name,
		  code_symbols.qualified_name,
		  code_symbols.signature_hash,
		  code_symbols.kind,
		  code_symbols.receiver,
		  code_symbols.start_line,
		  code_symbols.end_line,
		  code_symbols.start_byte,
		  code_symbols.end_byte,
		  code_symbols.hash,
		  code_symbols.exported,
		  code_symbols.lang,
		  code_symbols.index_epoch
		FROM code_symbol_search
		JOIN code_symbols
		  ON code_symbols.id = code_symbol_search.symbol_id
		WHERE code_symbol_search MATCH ?
		  AND published_epoch = ?`
	args := []any{expression, epoch}
	statement, args = appendSymbolFilterSQL(
		statement,
		args,
		"code_symbols.kind",
		filters.Kinds,
		symbolFilterEqual,
	)
	statement, args = appendSymbolFilterSQL(
		statement,
		args,
		"code_symbols.lang",
		filters.Langs,
		symbolFilterEqual,
	)
	statement, args = appendSymbolFilterSQL(
		statement,
		args,
		"code_symbols.file_path",
		filters.PathFilters,
		symbolFilterContains,
	)
	statement, args = appendSymbolFilterSQL(
		statement,
		args,
		"code_symbols.name",
		filters.NameFilters,
		symbolFilterContains,
	)
	statement += `
		ORDER BY code_symbols.anchor_id,
		         code_symbols.file_path,
		         code_symbols.qualified_name
		LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("query symbol search projection: %w", err)
	}
	defer rows.Close()
	return scanCodeSymbols(rows)
}

type symbolFilterMode struct {
	code string
}

var (
	symbolFilterEqual = symbolFilterMode{
		code: "equal",
	}
	symbolFilterContains = symbolFilterMode{
		code: "contains",
	}
)

func appendSymbolFilterSQL(
	statement string,
	args []any,
	column string,
	values []string,
	mode symbolFilterMode,
) (string, []any) {
	if len(values) == 0 {
		return statement, args
	}
	clauses := make([]string, 0, len(values))
	for _, value := range values {
		clause := "lower(" + column + ") = lower(?)"
		if mode.code == symbolFilterContains.code {
			clause = "instr(lower(" + column + "), lower(?)) > 0"
		}
		clauses = append(clauses, clause)
		args = append(args, value)
	}
	statement += " AND (" + strings.Join(clauses, " OR ") + ")"
	return statement, args
}

func symbolSearchExpression(terms []string) string {
	parts := make([]string, 0, len(terms))
	for _, term := range terms {
		escaped := strings.ReplaceAll(term, `"`, `""`)
		parts = append(parts, `"`+escaped+`"*`)
	}
	return strings.Join(parts, " OR ")
}

func symbolTermMatches(
	symbol CodeSymbol,
	terms []string,
) ([]SymbolTermMatch, int, TermCoverage) {
	fields := []struct {
		code  string
		value string
	}{
		{code: MatchFieldName, value: symbol.Name},
		{code: MatchFieldQualifiedName, value: symbol.QualifiedName},
		{code: MatchFieldReceiver, value: symbol.Receiver},
		{code: MatchFieldKind, value: symbol.Kind},
		{code: MatchFieldPath, value: symbol.FilePath},
	}
	fieldTerms := make([]map[string]bool, 0, len(fields))
	for _, field := range fields {
		values := textsearch.Terms(
			field.value,
			textsearch.Options{Stems: true},
		)
		index := make(map[string]bool, len(values))
		for _, value := range values {
			index[value] = true
		}
		fieldTerms = append(fieldTerms, index)
	}
	bestField := len(fields)
	fieldMatchCounts := make([]int, len(fields))
	matches := make([]SymbolTermMatch, 0, len(terms))
	for _, term := range terms {
		matchedFields := make([]MatchField, 0)
		for index, field := range fields {
			if !fieldTerms[index][term] {
				continue
			}
			matchedFields = append(
				matchedFields,
				MatchField{code: field.code},
			)
			if rank := matchFieldRank(field.code); rank < bestField {
				bestField = rank
			}
			fieldMatchCounts[index]++
		}
		if len(matchedFields) == 0 {
			continue
		}
		matches = append(
			matches,
			SymbolTermMatch{
				term:   term,
				fields: matchedFields,
			},
		)
	}
	bestFieldIndex := -1
	for index, field := range fields {
		if matchFieldRank(field.code) != bestField {
			continue
		}
		bestFieldIndex = index
		break
	}
	fieldCoverage := TermCoverage{}
	if bestFieldIndex >= 0 {
		fieldCoverage = TermCoverage{
			covered: fieldMatchCounts[bestFieldIndex],
			total:   len(fieldTerms[bestFieldIndex]),
		}
	}
	return matches, bestField, fieldCoverage
}

func matchFieldRank(field string) int {
	switch field {
	case MatchFieldName:
		return 0
	case MatchFieldQualifiedName:
		return 1
	case MatchFieldReceiver:
		return 2
	case MatchFieldKind:
		return 3
	case MatchFieldPath:
		return 4
	default:
		return 5
	}
}

func symbolFilterHits(
	symbol CodeSymbol,
	query textsearch.ParsedQuery,
) int {
	hits := 0
	if len(query.Kinds) > 0 && anyEqualFold(query.Kinds, symbol.Kind) {
		hits++
	}
	if len(query.Langs) > 0 && anyEqualFold(query.Langs, symbol.Lang) {
		hits++
	}
	if len(query.PathFilters) > 0 &&
		anySubstringFold(query.PathFilters, symbol.FilePath) {
		hits++
	}
	if len(query.NameFilters) > 0 &&
		anySubstringFold(query.NameFilters, symbol.Name) {
		hits++
	}
	return hits
}

func classifySymbolSourceLane(path string) SymbolSourceLane {
	switch {
	case textsearch.IsGeneratedPath(path):
		return SymbolSourceLane{code: SymbolLaneGenerated}
	case textsearch.IsTestPath(path):
		return SymbolSourceLane{code: SymbolLaneTest}
	default:
		return SymbolSourceLane{code: SymbolLaneProduction}
	}
}

func sortSymbolDiscoveryCandidates(
	candidates []SymbolDiscoveryCandidate,
) {
	sort.SliceStable(candidates, func(leftIndex, rightIndex int) bool {
		left := candidates[leftIndex]
		right := candidates[rightIndex]
		if leftTier, rightTier := lexicalTierRank(left.tier), lexicalTierRank(right.tier); leftTier != rightTier {
			return leftTier < rightTier
		}
		if left.coverage.covered != right.coverage.covered {
			return left.coverage.covered > right.coverage.covered
		}
		leftRatio := left.coverage.covered * right.coverage.total
		rightRatio := right.coverage.covered * left.coverage.total
		if leftRatio != rightRatio {
			return leftRatio > rightRatio
		}
		if left.bestField != right.bestField {
			return left.bestField < right.bestField
		}
		leftFieldRatio := left.fieldCoverage.covered *
			right.fieldCoverage.total
		rightFieldRatio := right.fieldCoverage.covered *
			left.fieldCoverage.total
		if leftFieldRatio != rightFieldRatio {
			return leftFieldRatio > rightFieldRatio
		}
		if left.filterHits != right.filterHits {
			return left.filterHits > right.filterHits
		}
		if left.symbol.AnchorID != right.symbol.AnchorID {
			return left.symbol.AnchorID < right.symbol.AnchorID
		}
		if left.symbol.FilePath != right.symbol.FilePath {
			return left.symbol.FilePath < right.symbol.FilePath
		}
		return left.symbol.QualifiedName < right.symbol.QualifiedName
	})
}

func lexicalTierRank(tier LexicalTier) int {
	switch tier.String() {
	case LexicalTierExactStableID:
		return 0
	case LexicalTierExactQualifiedName:
		return 1
	case LexicalTierExactName:
		return 2
	case LexicalTierAllTerms:
		return 3
	case LexicalTierPartialTerms:
		return 4
	case LexicalTierEditDistance:
		return 5
	default:
		return 6
	}
}

func symbolSearchDocument(value string) string {
	return strings.Join(
		textsearch.Terms(
			value,
			textsearch.Options{Stems: true},
		),
		" ",
	)
}

func insertSymbolSearchRow(
	ctx context.Context,
	executor interface {
		ExecContext(
			context.Context,
			string,
			...any,
		) (sql.Result, error)
	},
	symbol CodeSymbol,
	epoch int64,
) error {
	_, err := executor.ExecContext(ctx, `
		INSERT INTO code_symbol_search (
		  symbol_id, published_epoch, name, qualified_name,
		  receiver, kind, file_path
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		symbol.ID,
		epoch,
		symbolSearchDocument(symbol.Name),
		symbolSearchDocument(symbol.QualifiedName),
		symbolSearchDocument(symbol.Receiver),
		symbolSearchDocument(symbol.Kind),
		symbolSearchDocument(symbol.FilePath),
	)
	return err
}

func deleteSymbolSearchRowsByFile(
	ctx context.Context,
	executor interface {
		ExecContext(
			context.Context,
			string,
			...any,
		) (sql.Result, error)
	},
	path string,
) error {
	_, err := executor.ExecContext(
		ctx,
		`DELETE FROM code_symbol_search
		 WHERE symbol_id IN (
		   SELECT id FROM code_symbols WHERE file_path = ?
		 )`,
		path,
	)
	return err
}

// SetSymbolSearchEpoch moves every derivative row onto one published view
// epoch. Production publication calls the transaction-scoped equivalent.
func (s *SymbolStore) SetSymbolSearchEpoch(
	ctx context.Context,
	epoch int64,
) error {
	if epoch < 1 {
		return fmt.Errorf("symbol search epoch must be positive")
	}
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE code_symbol_search SET published_epoch = ?`,
		epoch,
	)
	return err
}

func (s *SymbolStore) SymbolSearchEpoch(
	ctx context.Context,
) (int64, error) {
	var minimum sql.NullInt64
	var maximum sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT MIN(published_epoch), MAX(published_epoch)
		FROM code_symbol_search`).Scan(&minimum, &maximum)
	if err != nil {
		return 0, err
	}
	if !minimum.Valid && !maximum.Valid {
		var currentEpoch int64
		metaErr := s.db.QueryRowContext(
			ctx,
			`SELECT current_epoch FROM code_index_meta WHERE id = 1`,
		).Scan(&currentEpoch)
		if metaErr == sql.ErrNoRows ||
			strings.Contains(
				fmt.Sprint(metaErr),
				"no such table",
			) {
			return 0, nil
		}
		return currentEpoch, metaErr
	}
	if minimum.Int64 != maximum.Int64 {
		return 0, fmt.Errorf(
			"search_epoch_mismatch: derivative rows span epochs %d..%d",
			minimum.Int64,
			maximum.Int64,
		)
	}
	return minimum.Int64, nil
}

// RebuildSymbolSearchProjection recreates the derivative entirely from the
// canonical symbol table without changing any symbol identity.
func (s *SymbolStore) RebuildSymbolSearchProjection(
	ctx context.Context,
	epoch int64,
) error {
	if epoch < 1 {
		return fmt.Errorf("symbol search epoch must be positive")
	}
	symbols, err := s.AllSymbols(ctx)
	if err != nil {
		return err
	}
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback() }()
	if _, err := transaction.ExecContext(
		ctx,
		`DELETE FROM code_symbol_search`,
	); err != nil {
		return err
	}
	for _, symbol := range symbols {
		if err := insertSymbolSearchRow(
			ctx,
			transaction,
			symbol,
			epoch,
		); err != nil {
			return err
		}
	}
	return transaction.Commit()
}
