package codebase

import (
	"regexp"
	"strings"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

const tsCallExpressionQuery = `(call_expression) @call`

type tsCompiledQueryKey struct {
	language *sitter.Language
	pattern  string
}

var tsCompiledQueries sync.Map

func compiledTSQuery(pattern string, language *sitter.Language) (*sitter.Query, error) {
	key := tsCompiledQueryKey{language: language, pattern: pattern}
	if cached, ok := tsCompiledQueries.Load(key); ok {
		return cached.(*sitter.Query), nil
	}
	query, err := sitter.NewQuery([]byte(pattern), language)
	if err != nil {
		return nil, err
	}
	actual, loaded := tsCompiledQueries.LoadOrStore(key, query)
	if loaded {
		query.Close()
	}
	return actual.(*sitter.Query), nil
}

// tsNameImport binds a name introduced by `import { Orig as Local } from './m'`
// to the resolved target-module base path and the original exported name.
type tsNameImport struct {
	bases    []string // candidate project-relative module paths without extension
	orig     string   // the name as exported by the target module; "default" for default imports
	external bool
}

// tsImports is the resolved relative-import surface of one JS/TS file:
//   - names:      local name -> {target base, original name} for `import { N }`
//   - namespaces: alias       -> target base                  for `import * as ns`
//
// Bare (non-relative) imports are external dependencies and are not recorded —
// they never resolve to a project node.
type tsImports struct {
	names              map[string]tsNameImport
	namespaces         map[string][]string
	externalNamespaces map[string]bool
	model              *tsProjectModel
}

type tsCallAnalysis struct {
	calls             []CallSite
	callbacks         []callbackRef
	receiverTypes     map[int]map[string]string
	emitterRegisters  []emitterReg
	emitterDispatches []emitterDispatch
}

// extractTSImports reads the `import ... from '<spec>'` statements of a JS/TS file
// and resolves each specifier to a project-relative base path (extension
// stripped): relative paths against the importing dir, plus tsconfig path aliases
// and monorepo workspace packages via projRes. A genuinely external specifier is
// skipped. Pure relative to (root, content, fileDir, projRes); caller owns the tree.
func extractTSImports(root *sitter.Node, content []byte, fileDir string, projRes tsProjectResolution) tsImports {
	res := tsImports{
		names:              map[string]tsNameImport{},
		namespaces:         map[string][]string{},
		externalNamespaces: map[string]bool{},
	}
	for i := 0; i < int(root.NamedChildCount()); i++ {
		n := root.NamedChild(i)
		if n.Type() != "import_statement" {
			continue
		}
		source := n.ChildByFieldName("source")
		if source == nil {
			continue
		}
		raw := strings.Trim(source.Content(content), "'\"`")
		bases, local := resolveTSModuleSpecifiers(raw, fileDir, projRes)
		clause := firstChildOfType(n, "import_clause")
		if clause == nil {
			continue
		}
		collectTSImportClause(clause, content, bases, !local, &res)
	}
	return res
}

// collectTSImportClause records the bindings of one import clause: named imports
// `{ A, B as C }`, a namespace `* as ns`, or a bare default (skipped — the export
// name is not knowable from the import site).
func collectTSImportClause(clause *sitter.Node, content []byte, bases []string, external bool, res *tsImports) {
	for i := 0; i < int(clause.NamedChildCount()); i++ {
		c := clause.NamedChild(i)
		switch c.Type() {
		case "identifier":
			local := c.Content(content)
			res.names[local] = tsNameImport{bases: append([]string{}, bases...), orig: "default", external: external}
		case "named_imports":
			for j := 0; j < int(c.NamedChildCount()); j++ {
				spec := c.NamedChild(j)
				if spec.Type() != "import_specifier" {
					continue
				}
				name := spec.ChildByFieldName("name")
				if name == nil {
					continue
				}
				local := name.Content(content)
				if alias := spec.ChildByFieldName("alias"); alias != nil {
					local = alias.Content(content)
				}
				res.names[local] = tsNameImport{bases: append([]string{}, bases...), orig: name.Content(content), external: external}
			}
		case "namespace_import":
			if id := firstChildOfType(c, "identifier"); id != nil {
				alias := id.Content(content)
				res.namespaces[alias] = append([]string{}, bases...)
				res.externalNamespaces[alias] = external
			}
		}
	}
}

// extractTSCallAnalysis walks a parsed JS/TS file's call expressions once. It
// derives calls, callback references, receiver facts, and emitter sites from
// the same captured nodes so the resolver does not repeat CGo tree traversal.
func extractTSCallAnalysis(root *sitter.Node, content []byte, lang *sitter.Language) tsCallAnalysis {
	q, err := compiledTSQuery(tsCallExpressionQuery, lang)
	if err != nil {
		return tsCallAnalysis{}
	}
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, root)
	bindings := indexTSCallableBindings(root, content)

	analysis := tsCallAnalysis{receiverTypes: map[int]map[string]string{}}
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, capture := range m.Captures {
			call := capture.Node
			appendTSEmitterSite(call, content, &analysis)
			fn := call.ChildByFieldName("function")
			if fn == nil {
				continue
			}
			var callee, qualifier string
			switch fn.Type() {
			case "identifier":
				callee = fn.Content(content)
			case "member_expression":
				prop := fn.ChildByFieldName("property")
				obj := fn.ChildByFieldName("object")
				if prop == nil || obj == nil {
					continue
				}
				callee = prop.Content(content)
				qualifier = obj.Content(content)
			default:
				continue
			}
			if callee == "" {
				continue
			}
			line := int(call.StartPoint().Row) + 1
			bindingName := callee
			if qualifier != "" {
				bindingName = strings.SplitN(qualifier, ".", 2)[0]
			}
			analysis.calls = append(analysis.calls, CallSite{
				Callee:    callee,
				Qualifier: qualifier,
				Line:      line,
				Shadowed:  tsNameBoundInEnclosingCallable(call, bindingName, bindings),
			})
			if qualifier != "" {
				receiverType := inferTSReceiverType(call, qualifier, content)
				if receiverType != "" {
					if analysis.receiverTypes[line] == nil {
						analysis.receiverTypes[line] = map[string]string{}
					}
					analysis.receiverTypes[line][qualifier] = receiverType
				}
			}
			if args := call.ChildByFieldName("arguments"); args != nil {
				for i := 0; i < int(args.NamedChildCount()); i++ {
					if a := args.NamedChild(i); a.Type() == "identifier" {
						name := a.Content(content)
						analysis.callbacks = append(analysis.callbacks, callbackRef{
							callee:   callee,
							name:     name,
							line:     line,
							shadowed: tsNameBoundInEnclosingCallable(call, name, bindings),
						})
					}
				}
			}
		}
	}
	return analysis
}

func inferTSReceiverType(call *sitter.Node, qualifier string, content []byte) string {
	if match := regexp.MustCompile(`^new\s+([A-Za-z_$][\w$]*)`).FindStringSubmatch(strings.TrimSpace(qualifier)); len(match) == 2 {
		return match[1]
	}
	callable := tsEnclosingCallableNode(call)
	if callable == nil {
		return ""
	}
	start := int(callable.StartByte())
	end := int(call.StartByte())
	if start < 0 || end <= start || end > len(content) {
		return ""
	}
	prefix := string(content[start:end])
	if strings.HasPrefix(qualifier, "this.") {
		field := strings.TrimPrefix(qualifier, "this.")
		classNode := tsEnclosingClassNode(callable)
		if classNode == nil {
			return ""
		}
		classText := classNode.Content(content)
		return firstTSAnnotatedType(classText, field)
	}
	if inferred := firstTSNewAssignmentType(prefix, qualifier); inferred != "" {
		return inferred
	}
	return firstTSAnnotatedType(prefix, qualifier)
}

func firstTSAnnotatedType(source, name string) string {
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*[!?]?\s*:\s*([A-Za-z_$][\w$]*)`)
	match := pattern.FindStringSubmatch(source)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func firstTSNewAssignmentType(source, name string) string {
	pattern := regexp.MustCompile(`\b(?:const|let|var)\s+` + regexp.QuoteMeta(name) + `(?:\s*:\s*[A-Za-z_$][\w$]*)?\s*=\s*new\s+([A-Za-z_$][\w$]*)`)
	match := pattern.FindStringSubmatch(source)
	if len(match) != 2 {
		return ""
	}
	return match[1]
}

func tsEnclosingClassNode(node *sitter.Node) *sitter.Node {
	current := node.Parent()
	for current != nil {
		switch current.Type() {
		case "class_declaration", "abstract_class_declaration":
			return current
		case "program":
			return nil
		}
		current = current.Parent()
	}
	return nil
}

// tsFileMatchesBase reports whether a stored file path is the module named by a
// project-relative base — either `<base>.<ext>` or `<base>/index.<ext>`.
func tsFileMatchesBase(file, base string) bool {
	fileBase := moduleBase(file)
	targetBase := moduleBase(base)
	return fileBase == targetBase || fileBase == targetBase+"/index"
}

// resolveTSImportedName finds the single func/class symbol named `name` defined in
// the target module base, or nil if zero/ambiguous. The cross-module primitive.
func resolveTSImportedName(name string, bases []string, imports tsImports, lookup func(string) []CodeSymbol) *CodeSymbol {
	cands := tsImportedCandidates(name, bases, imports, lookup)
	if len(cands) != 1 {
		return nil
	}
	return &cands[0]
}

func resolveTSCallOutcomes(
	relPath string,
	fileSyms []CodeSymbol,
	pkgSyms []CodeSymbol,
	calls []CallSite,
	callbacks []callbackRef,
	receiverTypes map[int]map[string]string,
	imports tsImports,
	lookup func(name string) []CodeSymbol,
	sourceHash string,
) []EdgeResolution {
	fileDefs := map[string][]CodeSymbol{}
	for _, s := range fileSyms {
		if s.Kind == "func" || s.Kind == "class" {
			fileDefs[s.Name] = append(fileDefs[s.Name], s)
		}
	}
	pkgDefs := map[string][]CodeSymbol{}
	for _, s := range pkgSyms {
		if s.Kind == "func" || s.Kind == "class" {
			pkgDefs[s.Name] = append(pkgDefs[s.Name], s)
		}
	}

	callPairs := map[string]bool{}
	var outcomes []EdgeResolution
	for _, cs := range calls {
		caller := enclosingSymbol(fileSyms, cs.Line)
		if caller == nil {
			continue
		}
		var candidates []CodeSymbol
		origin := EdgeOriginASTCall
		if cs.Qualifier == "" && cs.Shadowed {
			outcomes = append(outcomes, UnresolvedEdge{
				SourceID:           caller.ID,
				Kind:               EdgeCall,
				FilePath:           relPath,
				Line:               cs.Line,
				Reason:             ResolutionReasonShadowedBinding,
				Origin:             origin,
				ResolverVersion:    tsResolverVersion,
				SourceSnapshotHash: sourceHash,
			})
			continue
		}
		if cs.Qualifier == "" {
			candidates, origin = resolveTSUnqualifiedCandidates(cs.Callee, fileDefs, pkgDefs, imports, lookup)
		} else {
			receiverType := ""
			if receiverTypes[cs.Line] != nil {
				receiverType = receiverTypes[cs.Line][cs.Qualifier]
			}
			candidates, origin = resolveTSQualifiedCandidates(cs.Qualifier, cs.Callee, receiverType, !cs.Shadowed, *caller, fileSyms, imports, lookup)
		}
		if len(candidates) == 0 {
			outcomes = append(outcomes, UnresolvedEdge{
				SourceID:           caller.ID,
				Kind:               EdgeCall,
				FilePath:           relPath,
				Line:               cs.Line,
				Reason:             unresolvedTSCallReason(cs, imports),
				Origin:             origin,
				ResolverVersion:    tsResolverVersion,
				SourceSnapshotHash: sourceHash,
			})
			continue
		}
		if len(candidates) > 1 {
			candidateIDs := make([]string, 0, len(candidates))
			for _, candidate := range candidates {
				candidateIDs = append(candidateIDs, candidate.ID)
			}
			outcomes = append(outcomes, AmbiguousEdge{
				SourceID:           caller.ID,
				Kind:               EdgeCall,
				FilePath:           relPath,
				Line:               cs.Line,
				Reason:             ResolutionReasonMultipleCandidates,
				CandidateIDs:       candidateIDs,
				Origin:             origin,
				ResolverVersion:    tsResolverVersion,
				SourceSnapshotHash: sourceHash,
			})
			continue
		}
		dst := candidates[0]
		if dst.ID == caller.ID {
			continue
		}
		key := caller.ID + "->" + dst.ID
		if callPairs[key] {
			continue
		}
		callPairs[key] = true
		outcomes = append(outcomes, ResolvedEdge{Edge: CodeEdge{
			SrcID:              caller.ID,
			DstID:              dst.ID,
			Kind:               EdgeCall,
			FilePath:           relPath,
			Line:               cs.Line,
			Provenance:         ProvenanceStatic,
			Origin:             origin,
			ResolutionMethod:   ResolutionMethodExactSymbol,
			Confidence:         ConfidenceExact,
			ResolverVersion:    tsResolverVersion,
			SourceSnapshotHash: sourceHash,
		}})
	}

	resolve := func(name string) *CodeSymbol {
		return resolveTSUnqualified(name, fileDefs, pkgDefs, imports, lookup)
	}
	callbackEdges := synthesizeCallbackEdges(relPath, fileSyms, callbacks, callPairs, resolve)
	outcomes = append(outcomes, resolvedTSOutcomes(callbackEdges, sourceHash, EdgeOriginCallbackRegistration)...)
	return outcomes
}

func resolveTSUnqualifiedCandidates(
	callee string,
	fileDefs map[string][]CodeSymbol,
	pkgDefs map[string][]CodeSymbol,
	imports tsImports,
	lookup func(string) []CodeSymbol,
) ([]CodeSymbol, EdgeOrigin) {
	local := fileDefs[callee]
	binding, imported := imports.names[callee]
	if len(local) > 0 && imported {
		importedCandidates := tsImportedCandidates(binding.orig, binding.bases, imports, lookup)
		return append(append([]CodeSymbol{}, local...), importedCandidates...), EdgeOriginNamedImport
	}
	if len(local) > 0 {
		return append([]CodeSymbol{}, local...), EdgeOriginASTCall
	}
	if imported {
		return tsImportedCandidates(binding.orig, binding.bases, imports, lookup), EdgeOriginNamedImport
	}
	return nil, EdgeOriginASTCall
}

func resolveTSQualifiedCandidates(
	qualifier string,
	callee string,
	receiverType string,
	allowImportedQualifier bool,
	caller CodeSymbol,
	fileSymbols []CodeSymbol,
	imports tsImports,
	lookup func(string) []CodeSymbol,
) ([]CodeSymbol, EdgeOrigin) {
	if bases, namespace := imports.namespaces[qualifier]; namespace {
		return tsImportedCandidates(callee, bases, imports, lookup), EdgeOriginNamespaceImport
	}
	if qualifier == "this" && caller.Receiver != "" {
		return tsReceiverMethodCandidates(callee, caller.Receiver, caller.FilePath, lookup), EdgeOriginReceiverType
	}
	if allowImportedQualifier {
		if binding, imported := imports.names[qualifier]; imported && !binding.external {
			return tsImportedReceiverMethodCandidates(callee, binding, imports, lookup), EdgeOriginNamedImport
		}
	}
	if receiverType != "" {
		return tsTypedReceiverMethodCandidates(callee, receiverType, fileSymbols, imports, lookup), EdgeOriginReceiverType
	}
	containers := make([]CodeSymbol, 0)
	for _, symbol := range fileSymbols {
		if symbol.Name == qualifier && (symbol.Kind == "class" || symbol.Kind == "constant" || symbol.Kind == "variable") {
			containers = append(containers, symbol)
		}
	}
	if len(containers) != 1 {
		return nil, EdgeOriginReceiverType
	}
	return tsReceiverMethodCandidates(callee, containers[0].Name, containers[0].FilePath, lookup), EdgeOriginReceiverType
}

func tsTypedReceiverMethodCandidates(
	callee string,
	receiverType string,
	fileSymbols []CodeSymbol,
	imports tsImports,
	lookup func(string) []CodeSymbol,
) []CodeSymbol {
	if binding, imported := imports.names[receiverType]; imported && !binding.external {
		return tsImportedReceiverMethodCandidates(callee, binding, imports, lookup)
	}
	classes := make([]CodeSymbol, 0)
	for _, symbol := range fileSymbols {
		if symbol.Name == receiverType && symbol.Kind == "class" {
			classes = append(classes, symbol)
		}
	}
	if len(classes) != 1 {
		return nil
	}
	return tsReceiverMethodCandidates(callee, classes[0].Name, classes[0].FilePath, lookup)
}

func tsImportedReceiverMethodCandidates(
	callee string,
	binding tsNameImport,
	imports tsImports,
	lookup func(string) []CodeSymbol,
) []CodeSymbol {
	targets := make([]tsExportTarget, 0)
	for _, base := range binding.bases {
		if imports.model == nil {
			targets = append(targets, tsExportTarget{fileBase: base, symbolName: binding.orig})
			continue
		}
		targets = append(targets, imports.model.ResolveExport(base, binding.orig)...)
	}
	candidates := make([]CodeSymbol, 0)
	for _, target := range targets {
		candidates = append(candidates, tsReceiverMethodCandidates(callee, target.symbolName, target.fileBase, lookup)...)
	}
	return dedupeCodeSymbols(candidates)
}

func tsReceiverMethodCandidates(
	callee string,
	receiver string,
	fileBase string,
	lookup func(string) []CodeSymbol,
) []CodeSymbol {
	candidates := make([]CodeSymbol, 0)
	for _, symbol := range lookup(callee) {
		if symbol.Kind != "method" || symbol.Receiver != receiver || !tsFileMatchesBase(symbol.FilePath, fileBase) {
			continue
		}
		candidates = append(candidates, symbol)
	}
	return candidates
}

func tsImportedCandidates(name string, bases []string, imports tsImports, lookup func(string) []CodeSymbol) []CodeSymbol {
	targets := make([]tsExportTarget, 0)
	for _, base := range bases {
		if imports.model == nil {
			targets = append(targets, tsExportTarget{fileBase: base, symbolName: name})
			continue
		}
		targets = append(targets, imports.model.ResolveExport(base, name)...)
	}
	candidates := make([]CodeSymbol, 0)
	for _, target := range targets {
		for _, symbol := range lookup(target.symbolName) {
			if !callableTSSymbol(symbol) || !tsFileMatchesBase(symbol.FilePath, target.fileBase) {
				continue
			}
			candidates = append(candidates, symbol)
		}
	}
	return dedupeCodeSymbols(candidates)
}

func unresolvedTSCallReason(call CallSite, imports tsImports) ResolutionReason {
	if call.Qualifier == "" {
		if binding, imported := imports.names[call.Callee]; imported && binding.external {
			return ResolutionReasonExternalDependency
		}
	}
	if call.Qualifier != "" {
		if imports.externalNamespaces[call.Qualifier] {
			return ResolutionReasonExternalDependency
		}
		if _, namespace := imports.namespaces[call.Qualifier]; !namespace {
			return ResolutionReasonUnsupportedForm
		}
	}
	return ResolutionReasonNoCandidate
}

type tsCallableRange struct {
	start uint32
	end   uint32
}

type tsCallableBindingSet struct {
	parameters map[string]bool
	locals     map[string][]uint32
}

type tsCallableBindingIndex map[tsCallableRange]*tsCallableBindingSet

func indexTSCallableBindings(root *sitter.Node, content []byte) tsCallableBindingIndex {
	index := tsCallableBindingIndex{}
	var walk func(node *sitter.Node, current *tsCallableBindingSet)
	walk = func(node *sitter.Node, current *tsCallableBindingSet) {
		if node == nil {
			return
		}
		if tsCallableNode(node.Type()) {
			current = &tsCallableBindingSet{
				parameters: map[string]bool{},
				locals:     map[string][]uint32{},
			}
			parameters := node.ChildByFieldName("parameters")
			collectTSBindingNames(parameters, content, current.parameters)
			key := tsCallableRange{start: node.StartByte(), end: node.EndByte()}
			index[key] = current
		}
		if current != nil && node.Type() == "variable_declarator" {
			nameNode := node.ChildByFieldName("name")
			if nameNode != nil {
				name := nameNode.Content(content)
				current.locals[name] = append(current.locals[name], node.StartByte())
			}
		}
		for childIndex := 0; childIndex < int(node.NamedChildCount()); childIndex++ {
			walk(node.NamedChild(childIndex), current)
		}
	}
	walk(root, nil)
	return index
}

func collectTSBindingNames(node *sitter.Node, content []byte, names map[string]bool) {
	if node == nil {
		return
	}
	if node.Type() == "identifier" {
		names[node.Content(content)] = true
		return
	}
	for childIndex := 0; childIndex < int(node.NamedChildCount()); childIndex++ {
		collectTSBindingNames(node.NamedChild(childIndex), content, names)
	}
}

func tsNameBoundInEnclosingCallable(call *sitter.Node, name string, bindings tsCallableBindingIndex) bool {
	callable := tsEnclosingCallableNode(call)
	if callable == nil {
		return false
	}
	key := tsCallableRange{start: callable.StartByte(), end: callable.EndByte()}
	bindingSet := bindings[key]
	if bindingSet == nil {
		return false
	}
	if bindingSet.parameters[name] {
		return true
	}
	for _, start := range bindingSet.locals[name] {
		if start < call.StartByte() {
			return true
		}
	}
	return false
}

func tsEnclosingCallableNode(node *sitter.Node) *sitter.Node {
	current := node.Parent()
	for current != nil {
		switch current.Type() {
		case "function_declaration", "function_expression", "arrow_function", "method_definition":
			return current
		case "program":
			return nil
		}
		current = current.Parent()
	}
	return nil
}

func tsCallableNode(nodeType string) bool {
	switch nodeType {
	case "function_declaration", "function_expression", "arrow_function", "method_definition":
		return true
	default:
		return false
	}
}

// emitterReg is a `x.on("event", handler)` registration; emitterDispatch is a
// `x.emit("event")` site. Paired by event name, they synthesize a dispatcher ->
// handler edge for the dynamic dispatch the AST cannot follow directly.
type emitterReg struct {
	event   string
	handler string
	line    int
}

type emitterDispatch struct {
	event string
	line  int
}

var emitterRegisterNames = map[string]bool{"on": true, "once": true, "addlistener": true, "addeventlistener": true}
var emitterDispatchNames = map[string]bool{"emit": true, "fire": true, "dispatchevent": true, "trigger": true}

const emitterFanoutCap = 6 // skip an event with more handlers/dispatchers than this — too generic to pair confidently

func appendTSEmitterSite(call *sitter.Node, content []byte, analysis *tsCallAnalysis) {
	fn := call.ChildByFieldName("function")
	if fn == nil || fn.Type() != "member_expression" {
		return
	}
	property := fn.ChildByFieldName("property")
	arguments := call.ChildByFieldName("arguments")
	if property == nil || arguments == nil || arguments.NamedChildCount() == 0 {
		return
	}
	firstArgument := arguments.NamedChild(0)
	if firstArgument.Type() != "string" {
		return
	}
	event := strings.Trim(firstArgument.Content(content), "'\"`")
	if event == "" {
		return
	}
	name := strings.ToLower(property.Content(content))
	line := int(call.StartPoint().Row) + 1
	if emitterRegisterNames[name] && arguments.NamedChildCount() >= 2 {
		handler := tsHandlerName(arguments.NamedChild(1), content)
		if handler != "" {
			analysis.emitterRegisters = append(analysis.emitterRegisters, emitterReg{event: event, handler: handler, line: line})
		}
		return
	}
	if emitterDispatchNames[name] {
		analysis.emitterDispatches = append(analysis.emitterDispatches, emitterDispatch{event: event, line: line})
	}
}

// tsHandlerName names a handler argument: a bare identifier, or the property of a
// `this.method` / `obj.method` member reference. An inline arrow/function literal
// is anonymous and yields "".
func tsHandlerName(n *sitter.Node, content []byte) string {
	switch n.Type() {
	case "identifier":
		return n.Content(content)
	case "member_expression":
		if prop := n.ChildByFieldName("property"); prop != nil {
			return prop.Content(content)
		}
	}
	return ""
}

// synthesizeEmitterEdges pairs same-event registrations and dispatches WITHIN a
// file: for each event (fan-out capped), each dispatch site's enclosing function
// gets a heuristic EdgeCallback to each handler that resolves to exactly one
// file-local function/method. Generic high-fan-out events (error/data/...) are
// skipped — they need cross-instance type info to pair soundly.
func synthesizeEmitterEdges(relPath string, fileSyms []CodeSymbol, regs []emitterReg, dispatches []emitterDispatch, callPairs map[string]bool) []CodeEdge {
	handlersByEvent := map[string][]string{}
	for _, r := range regs {
		handlersByEvent[r.event] = append(handlersByEvent[r.event], r.handler)
	}
	dispatchesByEvent := map[string][]emitterDispatch{}
	for _, d := range dispatches {
		dispatchesByEvent[d.event] = append(dispatchesByEvent[d.event], d)
	}
	fileFns := map[string][]CodeSymbol{}
	for _, s := range fileSyms {
		if s.Kind == "func" || s.Kind == "method" {
			fileFns[s.Name] = append(fileFns[s.Name], s)
		}
	}

	seen := map[string]bool{}
	var edges []CodeEdge
	for event, handlers := range handlersByEvent {
		disps := dispatchesByEvent[event]
		if len(disps) == 0 || len(handlers) > emitterFanoutCap || len(disps) > emitterFanoutCap {
			continue
		}
		for _, h := range handlers {
			hs := fileFns[h]
			if len(hs) != 1 {
				continue // unresolved or ambiguous handler — drop, don't guess
			}
			for _, d := range disps {
				caller := enclosingSymbol(fileSyms, d.line)
				if caller == nil || caller.ID == hs[0].ID {
					continue
				}
				key := caller.ID + "->" + hs[0].ID
				if seen[key] || callPairs[key] {
					continue
				}
				seen[key] = true
				edges = append(edges, CodeEdge{
					SrcID:      caller.ID,
					DstID:      hs[0].ID,
					Kind:       EdgeCallback,
					FilePath:   relPath,
					Line:       d.line,
					Provenance: ProvenanceHeuristic,
				})
			}
		}
	}
	return edges
}

func resolveTSUnqualified(callee string, fileDefs, pkgDefs map[string][]CodeSymbol, imports tsImports, lookup func(string) []CodeSymbol) *CodeSymbol {
	local := fileDefs[callee]
	b, imported := imports.names[callee]
	if len(local) == 1 && !imported {
		return &local[0]
	}
	if imported {
		return resolveTSImportedName(b.orig, b.bases, imports, lookup)
	}
	return nil
}

func callableTSSymbol(symbol CodeSymbol) bool {
	switch symbol.Kind {
	case "func", "class", "method":
		return true
	default:
		return false
	}
}

func dedupeCodeSymbols(symbols []CodeSymbol) []CodeSymbol {
	seen := make(map[string]bool)
	out := make([]CodeSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		if seen[symbol.ID] {
			continue
		}
		seen[symbol.ID] = true
		out = append(out, symbol)
	}
	return out
}
