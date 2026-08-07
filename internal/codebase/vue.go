package codebase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"

	sitter "github.com/smacker/go-tree-sitter"
)

const (
	VueParseIndexed  = "indexed"
	VueParseEmpty    = "empty"
	VueParseDegraded = "degraded"
)

type VueLang struct{}

func (v *VueLang) Language() string             { return "vue" }
func (v *VueLang) Extensions() []string         { return []string{".vue"} }
func (v *VueLang) SymbolLanguage(string) string { return "vue" }

type VueParseStatus struct {
	Status       string
	ScriptBlocks int
	HasTemplate  bool
	Reason       string
}

var (
	vueScriptPattern     = regexp.MustCompile(`(?is)<script\b[^>]*>(.*?)</script\s*>`)
	vueScriptOpenPattern = regexp.MustCompile(`(?is)<script\b`)
	vueTemplatePattern   = regexp.MustCompile(`(?is)<template\b[^>]*>(.*?)</template\s*>`)
	vueIdentifierPattern = regexp.MustCompile(`[A-Za-z_$][\w$]*`)
)

func (v *VueLang) ExtractSymbolSnapshots(
	source AdmittedSource,
) ([]SymbolSnapshot, error) {
	return v.ExtractSymbolSnapshotsContext(context.Background(), source)
}

func (v *VueLang) ExtractSymbolSnapshotsContext(
	ctx context.Context,
	source AdmittedSource,
) ([]SymbolSnapshot, error) {
	relPath := source.Path().String()
	content := source.bytes()
	projection, scripts := vueScriptProjection(content)
	if scripts == 0 {
		return vueTemplateSnapshots(relPath, content), nil
	}
	snapshots, err := extractTSSymbolSnapshotsFromContentContext(
		ctx,
		relPath,
		projection,
		tsLanguage(),
	)
	if err != nil {
		return nil, err
	}
	snapshots = append(snapshots, vueTemplateSnapshots(relPath, content)...)
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].StartByte < snapshots[j].StartByte })
	return snapshots, nil
}

func (v *VueLang) ResolveFileEdges(ctx context.Context, projectRoot, relPath string, symbols SymbolView) ([]CodeEdge, error) {
	outcomes, err := v.ResolveFileEdgeOutcomes(ctx, projectRoot, relPath, symbols)
	if err != nil {
		return nil, err
	}
	edges, _ := PartitionEdgeResolutions(outcomes)
	return edges, nil
}

func (v *VueLang) ResolveFileEdgeOutcomes(ctx context.Context, projectRoot, relPath string, symbols SymbolView) ([]EdgeResolution, error) {
	source, err := NewRegistry().ReadAdmittedSource(projectRoot, relPath)
	if err != nil {
		return nil, err
	}
	return v.ResolveAdmittedFileEdgeOutcomes(
		ctx,
		projectRoot,
		source,
		symbols,
	)
}

func (v *VueLang) ResolveFileEdgeOutcomesWithProjectSnapshot(
	ctx context.Context,
	projectRoot string,
	relPath string,
	symbols SymbolView,
	snapshot *projectIndexSnapshot,
) ([]EdgeResolution, error) {
	source, err := NewRegistry().ReadAdmittedSource(projectRoot, relPath)
	if err != nil {
		return nil, err
	}
	return v.ResolveAdmittedFileEdgeOutcomesWithProjectSnapshot(
		ctx,
		projectRoot,
		source,
		symbols,
		snapshot,
	)
}

func (v *VueLang) ResolveAdmittedFileEdgeOutcomes(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	symbols SymbolView,
) ([]EdgeResolution, error) {
	return v.resolveAdmittedFileEdgeOutcomes(
		ctx,
		projectRoot,
		source,
		symbols,
		nil,
	)
}

func (v *VueLang) ResolveAdmittedFileEdgeOutcomesWithProjectSnapshot(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	symbols SymbolView,
	snapshot *projectIndexSnapshot,
) ([]EdgeResolution, error) {
	return v.resolveAdmittedFileEdgeOutcomes(
		ctx,
		projectRoot,
		source,
		symbols,
		snapshot.typescript,
	)
}

func (v *VueLang) resolveAdmittedFileEdgeOutcomes(
	ctx context.Context,
	projectRoot string,
	source AdmittedSource,
	symbols SymbolView,
	snapshot *tsProjectSnapshot,
) ([]EdgeResolution, error) {
	relPath := source.Path().String()
	content := source.bytes()
	projection, scripts := vueScriptProjection(content)
	sourceHash := "sha256:" + source.Digest()
	outcomes := make([]EdgeResolution, 0)
	if scripts > 0 {
		tsOutcomes, err := resolveTSContentEdgeOutcomes(ctx, projectRoot, relPath, projection, sourceHash, tsLanguage(), symbols, snapshot)
		if err != nil {
			return nil, err
		}
		outcomes = append(outcomes, tsOutcomes...)
	}
	templateEdges, err := resolveVueTemplateUses(ctx, relPath, content, symbols, sourceHash)
	if err != nil {
		return nil, err
	}
	outcomes = append(outcomes, templateEdges...)
	return dedupeEdgeOutcomes(outcomes), nil
}

func InspectVueParse(projectRoot, relPath string) VueParseStatus {
	source, err := NewRegistry().ReadAdmittedSource(projectRoot, relPath)
	if err != nil {
		return VueParseStatus{Status: VueParseDegraded, Reason: err.Error()}
	}
	return InspectVueAdmittedParse(source)
}

func InspectVueAdmittedParse(source AdmittedSource) VueParseStatus {
	return InspectVueAdmittedParseContext(context.Background(), source)
}

func InspectVueAdmittedParseContext(
	ctx context.Context,
	source AdmittedSource,
) VueParseStatus {
	content := source.bytes()
	_, scripts := vueScriptProjection(content)
	hasTemplate := vueTemplatePattern.Match(content)
	if scripts == 0 && vueScriptOpenPattern.Match(content) {
		return VueParseStatus{Status: VueParseDegraded, HasTemplate: hasTemplate, Reason: "unterminated script block"}
	}
	if scripts == 0 && !hasTemplate {
		return VueParseStatus{Status: VueParseEmpty, Reason: "no script or template block"}
	}
	if scripts > 0 {
		projection, _ := vueScriptProjection(content)
		parser := sitter.NewParser()
		parser.SetLanguage(tsLanguage())
		tree, err := parser.ParseCtx(ctx, nil, projection)
		if err != nil {
			return VueParseStatus{Status: VueParseDegraded, ScriptBlocks: scripts, HasTemplate: hasTemplate, Reason: err.Error()}
		}
		degraded := tree.RootNode().HasError()
		tree.Close()
		if degraded {
			return VueParseStatus{Status: VueParseDegraded, ScriptBlocks: scripts, HasTemplate: hasTemplate, Reason: "tree-sitter reported syntax errors"}
		}
	}
	return VueParseStatus{Status: VueParseIndexed, ScriptBlocks: scripts, HasTemplate: hasTemplate}
}

func vueScriptProjection(content []byte) ([]byte, int) {
	projection := make([]byte, len(content))
	for index, value := range content {
		if value == '\n' || value == '\r' {
			projection[index] = value
		} else {
			projection[index] = ' '
		}
	}
	matches := vueScriptPattern.FindAllSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		copy(projection[match[2]:match[3]], content[match[2]:match[3]])
	}
	return projection, len(matches)
}

func vueTemplateSnapshots(relPath string, content []byte) []SymbolSnapshot {
	match := vueTemplatePattern.FindSubmatchIndex(content)
	if len(match) < 4 {
		return nil
	}
	start := match[2]
	end := match[3]
	body := content[start:end]
	hash := sha256.Sum256(body)
	qualifiedName := "__template__"
	return []SymbolSnapshot{{
		FilePath:      relPath,
		SymbolName:    qualifiedName,
		SymbolKind:    "template",
		QualifiedName: qualifiedName,
		SignatureHash: signatureHash("template", qualifiedName, ""),
		Line:          byteLine(content, start),
		EndLine:       byteLine(content, end),
		Hash:          hex.EncodeToString(hash[:]),
		StartByte:     start,
		EndByte:       end,
	}}
}

func resolveVueTemplateUses(
	ctx context.Context,
	relPath string,
	content []byte,
	symbols SymbolView,
	sourceHash string,
) ([]EdgeResolution, error) {
	fileSymbols, err := symbols.GetByFile(ctx, relPath)
	if err != nil {
		return nil, err
	}
	var template CodeSymbol
	byName := map[string][]CodeSymbol{}
	for _, symbol := range fileSymbols {
		if symbol.Kind == "template" {
			template = symbol
			continue
		}
		if symbol.Receiver == "" && (symbol.Kind == "func" || symbol.Kind == "constant" || symbol.Kind == "variable") {
			byName[symbol.Name] = append(byName[symbol.Name], symbol)
		}
	}
	if template.ID == "" {
		return nil, nil
	}
	match := vueTemplatePattern.FindSubmatch(content)
	if len(match) < 2 {
		return nil, nil
	}
	seen := map[string]bool{}
	outcomes := make([]EdgeResolution, 0)
	for _, token := range vueIdentifierPattern.FindAllString(string(match[1]), -1) {
		candidates := byName[token]
		if len(candidates) != 1 || seen[candidates[0].ID] {
			continue
		}
		seen[candidates[0].ID] = true
		outcomes = append(outcomes, ResolvedEdge{Edge: CodeEdge{
			SrcID:              template.ID,
			DstID:              candidates[0].ID,
			Kind:               EdgeTemplateUse,
			FilePath:           relPath,
			Line:               template.StartLine,
			Provenance:         ProvenanceStatic,
			Origin:             EdgeOriginVueTemplate,
			ResolutionMethod:   ResolutionMethodExactSymbol,
			Confidence:         ConfidenceExact,
			ResolverVersion:    tsResolverVersion,
			SourceSnapshotHash: sourceHash,
		}})
	}
	return outcomes, nil
}

func byteLine(content []byte, offset int) int {
	line := 1
	for index := 0; index < offset && index < len(content); index++ {
		if content[index] == '\n' {
			line++
		}
	}
	return line
}

func tsLanguage() *sitter.Language { return tsGrammarForExt(".ts") }
