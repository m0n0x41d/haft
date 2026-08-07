package codebase

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/projectpath"
)

const (
	defaultMaxFileBytes     = int64(500_000)
	defaultMaxFiles         = int64(100_000)
	defaultMaxObservedBytes = int64(2_000_000_000)
	defaultMaxParseWorkers  = int64(8)
)

var ErrSourceChanged = errors.New("source changed during bounded read")

// ByteCount is a validated non-negative byte quantity.
type ByteCount struct {
	value int64
}

func NewByteCount(value int64) (ByteCount, error) {
	if value < 0 {
		return ByteCount{}, fmt.Errorf("byte count cannot be negative")
	}
	return ByteCount{value: value}, nil
}

func (c ByteCount) Value() int64 {
	return c.value
}

func (c ByteCount) valid() bool {
	return c.value >= 0
}

// FileCount is a validated non-negative file quantity.
type FileCount struct {
	value int64
}

func NewFileCount(value int64) (FileCount, error) {
	if value < 0 {
		return FileCount{}, fmt.Errorf("file count cannot be negative")
	}
	return FileCount{value: value}, nil
}

func (c FileCount) Value() int64 {
	return c.value
}

func (c FileCount) valid() bool {
	return c.value >= 0
}

// WorkerCount is a validated positive parser-worker quantity.
type WorkerCount struct {
	value int64
}

func NewWorkerCount(value int64) (WorkerCount, error) {
	if value < 1 {
		return WorkerCount{}, fmt.Errorf("worker count must be positive")
	}
	return WorkerCount{value: value}, nil
}

func (c WorkerCount) Value() int64 {
	return c.value
}

func (c WorkerCount) valid() bool {
	return c.value > 0
}

// IndexBudgetSpec is the builder input for one validated admission policy.
type IndexBudgetSpec struct {
	MaxFileBytes     ByteCount
	MaxFiles         FileCount
	MaxObservedBytes ByteCount
	MaxParseWorkers  WorkerCount
	GeneratedSources GeneratedSourcePolicy
}

// GeneratedSourcePolicy keeps the existing include-and-deprioritize contract
// explicit while allowing a bounded caller to request typed exclusion.
type GeneratedSourcePolicy struct {
	code string
}

var generatedSourcePolicies = map[string]GeneratedSourcePolicy{
	"include_generated": {code: "include_generated"},
	"exclude_generated": {code: "exclude_generated"},
}

func ParseGeneratedSourcePolicy(
	raw string,
) (GeneratedSourcePolicy, error) {
	policy, found := generatedSourcePolicies[raw]
	if !found {
		return GeneratedSourcePolicy{}, fmt.Errorf(
			"unknown generated-source policy %q",
			raw,
		)
	}
	return policy, nil
}

func (p GeneratedSourcePolicy) String() string {
	return p.code
}

func (p GeneratedSourcePolicy) valid() bool {
	_, found := generatedSourcePolicies[p.code]
	return found
}

// IndexBudget is the single source-admission resource policy.
type IndexBudget struct {
	maxFileBytes     ByteCount
	maxFiles         FileCount
	maxObservedBytes ByteCount
	maxParseWorkers  WorkerCount
	generatedSources GeneratedSourcePolicy
}

func NewIndexBudget(spec IndexBudgetSpec) (IndexBudget, error) {
	if !spec.MaxFileBytes.valid() || spec.MaxFileBytes.value < 1 {
		return IndexBudget{}, fmt.Errorf("max file bytes must be positive")
	}
	if !spec.MaxFiles.valid() || spec.MaxFiles.value < 1 {
		return IndexBudget{}, fmt.Errorf("max files must be positive")
	}
	if !spec.MaxObservedBytes.valid() ||
		spec.MaxObservedBytes.value < spec.MaxFileBytes.value {
		return IndexBudget{}, fmt.Errorf(
			"max observed bytes must cover at least one maximum-size file",
		)
	}
	if !spec.MaxParseWorkers.valid() {
		return IndexBudget{}, fmt.Errorf("max parse workers must be positive")
	}
	if !spec.GeneratedSources.valid() {
		return IndexBudget{}, fmt.Errorf(
			"generated-source policy must be explicit",
		)
	}
	return IndexBudget{
		maxFileBytes:     spec.MaxFileBytes,
		maxFiles:         spec.MaxFiles,
		maxObservedBytes: spec.MaxObservedBytes,
		maxParseWorkers:  spec.MaxParseWorkers,
		generatedSources: spec.GeneratedSources,
	}, nil
}

func DefaultIndexBudget() IndexBudget {
	maxFileBytes, _ := NewByteCount(defaultMaxFileBytes)
	maxFiles, _ := NewFileCount(defaultMaxFiles)
	maxObservedBytes, _ := NewByteCount(defaultMaxObservedBytes)
	maxParseWorkers, _ := NewWorkerCount(defaultMaxParseWorkers)
	generatedSources, _ := ParseGeneratedSourcePolicy("include_generated")
	budget, _ := NewIndexBudget(IndexBudgetSpec{
		MaxFileBytes:     maxFileBytes,
		MaxFiles:         maxFiles,
		MaxObservedBytes: maxObservedBytes,
		MaxParseWorkers:  maxParseWorkers,
		GeneratedSources: generatedSources,
	})
	return budget
}

func (b IndexBudget) MaxFileBytes() ByteCount {
	return b.maxFileBytes
}

func (b IndexBudget) MaxFiles() FileCount {
	return b.maxFiles
}

func (b IndexBudget) MaxObservedBytes() ByteCount {
	return b.maxObservedBytes
}

func (b IndexBudget) MaxParseWorkers() WorkerCount {
	return b.maxParseWorkers
}

func (b IndexBudget) GeneratedSources() GeneratedSourcePolicy {
	return b.generatedSources
}

func (b IndexBudget) valid() bool {
	_, err := NewIndexBudget(IndexBudgetSpec{
		MaxFileBytes:     b.maxFileBytes,
		MaxFiles:         b.maxFiles,
		MaxObservedBytes: b.maxObservedBytes,
		MaxParseWorkers:  b.maxParseWorkers,
		GeneratedSources: b.generatedSources,
	})
	return err == nil
}

// ProjectPath is one normalized project-relative path.
type ProjectPath struct {
	value string
}

func NewProjectPath(value string) (ProjectPath, error) {
	parsed, err := projectpath.Parse(value)
	if err != nil {
		return ProjectPath{}, err
	}
	return ProjectPath{value: parsed.String()}, nil
}

func (p ProjectPath) String() string {
	return p.value
}

func (p ProjectPath) valid() bool {
	parsed, err := NewProjectPath(p.value)
	return err == nil && parsed.value == p.value
}

// SourceLanguage is a validated adapter-language identity.
type SourceLanguage struct {
	value string
}

func NewSourceLanguage(value string) (SourceLanguage, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return SourceLanguage{}, fmt.Errorf("source language must be canonical text")
	}
	return SourceLanguage{value: value}, nil
}

func (l SourceLanguage) String() string {
	return l.value
}

func (l SourceLanguage) valid() bool {
	return l.value != "" && strings.TrimSpace(l.value) == l.value
}

// SourceClass records why an observed path is or is not an ordinary supported
// source candidate before resource admission.
type SourceClass struct {
	code string
}

var sourceClasses = map[string]SourceClass{
	"supported":            {code: "supported"},
	"unsupported_language": {code: "unsupported_language"},
	"ignored_path":         {code: "ignored_path"},
	"generated_source":     {code: "generated_source"},
}

func ParseSourceClass(raw string) (SourceClass, error) {
	class, found := sourceClasses[raw]
	if !found {
		return SourceClass{}, fmt.Errorf("unknown source class %q", raw)
	}
	return class, nil
}

func (c SourceClass) String() string {
	return c.code
}

func (c SourceClass) valid() bool {
	_, found := sourceClasses[c.code]
	return found
}

type observedPayload interface {
	observedBytes() ByteCount
	content() ([]byte, bool)
	observedPayload()
}

type observedContent struct {
	bytes []byte
}

func (observedContent) observedPayload() {}

func (c observedContent) observedBytes() ByteCount {
	count, _ := NewByteCount(int64(len(c.bytes)))
	return count
}

func (c observedContent) content() ([]byte, bool) {
	return c.bytes, true
}

type observedMetadata struct {
	size ByteCount
}

func (observedMetadata) observedPayload() {}

func (m observedMetadata) observedBytes() ByteCount {
	return m.size
}

func (observedMetadata) content() ([]byte, bool) {
	return nil, false
}

// SourceObservation is a weak filesystem observation parsed into strong path,
// language, classification, and bounded-content carriers.
type SourceObservation struct {
	path     ProjectPath
	language SourceLanguage
	class    SourceClass
	payload  observedPayload
}

func NewContentObservation(
	path ProjectPath,
	language SourceLanguage,
	class SourceClass,
	content []byte,
) (SourceObservation, error) {
	if !path.valid() || !language.valid() || !class.valid() {
		return SourceObservation{}, fmt.Errorf("source observation identity is invalid")
	}
	return SourceObservation{
		path:     path,
		language: language,
		class:    class,
		payload:  observedContent{bytes: append([]byte(nil), content...)},
	}, nil
}

func NewMetadataObservation(
	path ProjectPath,
	language SourceLanguage,
	class SourceClass,
	observedBytes ByteCount,
) (SourceObservation, error) {
	if !path.valid() ||
		!language.valid() ||
		!class.valid() ||
		!observedBytes.valid() {
		return SourceObservation{}, fmt.Errorf("source metadata observation is invalid")
	}
	return SourceObservation{
		path:     path,
		language: language,
		class:    class,
		payload:  observedMetadata{size: observedBytes},
	}, nil
}

func (o SourceObservation) valid() bool {
	return o.path.valid() &&
		o.language.valid() &&
		o.class.valid() &&
		o.payload != nil
}

// AdmissionUsage is immutable root-level source workload already observed far
// enough to reach a content or per-file-size disposition. Pre-classified
// unsupported, ignored, and excluded-generated paths do not consume it.
type AdmissionUsage struct {
	files FileCount
	bytes ByteCount
}

func EmptyAdmissionUsage() AdmissionUsage {
	files, _ := NewFileCount(0)
	bytes, _ := NewByteCount(0)
	return AdmissionUsage{files: files, bytes: bytes}
}

func (u AdmissionUsage) Files() FileCount {
	return u.files
}

func (u AdmissionUsage) Bytes() ByteCount {
	return u.bytes
}

func (u AdmissionUsage) valid() bool {
	return u.files.valid() && u.bytes.valid()
}

// SourceSkipReason is a closed non-parser reason for excluding one source.
type SourceSkipReason struct {
	code string
}

var sourceSkipReasons = map[string]SourceSkipReason{
	"oversized":            {code: "oversized"},
	"invalid_encoding":     {code: "invalid_encoding"},
	"generated_source":     {code: "generated_source"},
	"unsupported_language": {code: "unsupported_language"},
	"ignored_path":         {code: "ignored_path"},
	"read_failure":         {code: "read_failure"},
	"source_changed":       {code: "source_changed"},
	"root_file_budget":     {code: "root_file_budget"},
	"root_byte_budget":     {code: "root_byte_budget"},
}

func ParseSourceSkipReason(raw string) (SourceSkipReason, error) {
	reason, found := sourceSkipReasons[raw]
	if !found {
		return SourceSkipReason{}, fmt.Errorf("unknown source skip reason %q", raw)
	}
	return reason, nil
}

func (r SourceSkipReason) String() string {
	return r.code
}

func (r SourceSkipReason) valid() bool {
	_, found := sourceSkipReasons[r.code]
	return found
}

// AdmittedSource is the only parser input. Its bytes and digest are private so
// production adapters cannot construct or mutate unaccounted parser input.
type AdmittedSource struct {
	path     ProjectPath
	language SourceLanguage
	content  []byte
	digest   string
}

func (s AdmittedSource) Path() ProjectPath {
	return s.path
}

func (s AdmittedSource) Language() SourceLanguage {
	return s.language
}

func (s AdmittedSource) Digest() string {
	return s.digest
}

func (s AdmittedSource) ByteCount() ByteCount {
	count, _ := NewByteCount(int64(len(s.content)))
	return count
}

func (s AdmittedSource) bytes() []byte {
	return s.content
}

func (s AdmittedSource) valid() bool {
	if !s.path.valid() || !s.language.valid() {
		return false
	}
	sum := sha256.Sum256(s.content)
	return s.digest == hex.EncodeToString(sum[:])
}

// SourceAdmissionKind is the closed discriminator for source admission.
type SourceAdmissionKind struct {
	code string
}

var sourceAdmissionKinds = map[string]SourceAdmissionKind{
	"source_admitted": {code: "source_admitted"},
	"source_skipped":  {code: "source_skipped"},
}

func (k SourceAdmissionKind) String() string {
	return k.code
}

// SourceAdmission is the sealed pre-parser result.
type SourceAdmission interface {
	Kind() SourceAdmissionKind
	DetailCode() string
	sourceAdmission()
}

type sourceAdmitted struct {
	source AdmittedSource
}

func (sourceAdmitted) sourceAdmission() {}

func (sourceAdmitted) Kind() SourceAdmissionKind {
	return sourceAdmissionKinds["source_admitted"]
}

func (sourceAdmitted) DetailCode() string {
	return "admitted"
}

type sourceSkipped struct {
	path          ProjectPath
	reason        SourceSkipReason
	observedBytes ByteCount
	limit         ByteCount
	detail        string
}

func (sourceSkipped) sourceAdmission() {}

func (sourceSkipped) Kind() SourceAdmissionKind {
	return sourceAdmissionKinds["source_skipped"]
}

func (s sourceSkipped) DetailCode() string {
	return s.reason.String()
}

// SourceSkipInfo is the external, immutable projection of a skipped source.
type SourceSkipInfo struct {
	Path          string
	Reason        string
	ObservedBytes int64
	LimitBytes    int64
	Detail        string
}

// RequiresRetry distinguishes an incomplete filesystem observation from a
// deliberate, inspectable policy exclusion.
func (i SourceSkipInfo) RequiresRetry() bool {
	return i.Reason == "read_failure" ||
		i.Reason == "source_changed" ||
		i.Reason == "root_file_budget" ||
		i.Reason == "root_byte_budget"
}

func SkippedSourceInfo(
	admission SourceAdmission,
) (SourceSkipInfo, error) {
	skipped, ok := admission.(sourceSkipped)
	if !ok {
		return SourceSkipInfo{}, fmt.Errorf("source admission is not skipped")
	}
	return SourceSkipInfo{
		Path:          skipped.path.String(),
		Reason:        skipped.reason.String(),
		ObservedBytes: skipped.observedBytes.Value(),
		LimitBytes:    skipped.limit.Value(),
		Detail:        skipped.detail,
	}, nil
}

func AdmittedSourceFrom(
	admission SourceAdmission,
) (AdmittedSource, error) {
	admitted, ok := admission.(sourceAdmitted)
	if !ok || !admitted.source.valid() {
		return AdmittedSource{}, fmt.Errorf("source admission is not admitted")
	}
	return admitted.source, nil
}

// AdmitSource is the pure source-admission core. It performs no filesystem,
// parser, database, or clock work.
func AdmitSource(
	observation SourceObservation,
	budget IndexBudget,
	usage AdmissionUsage,
) (SourceAdmission, AdmissionUsage, error) {
	if !observation.valid() || !budget.valid() || !usage.valid() {
		return nil, usage, fmt.Errorf("source admission inputs are invalid")
	}
	observedBytes := observation.payload.observedBytes()
	classReason := map[string]string{
		"unsupported_language": "unsupported_language",
		"ignored_path":         "ignored_path",
	}
	if budget.GeneratedSources().String() == "exclude_generated" {
		classReason["generated_source"] = "generated_source"
	}
	if reasonCode := classReason[observation.class.String()]; reasonCode != "" {
		reason := sourceSkipReasons[reasonCode]
		return newSkippedAdmission(
			observation.path,
			reason,
			observedBytes,
			ByteCount{},
			"classified before parser admission",
			usage,
		)
	}
	if usage.Files().Value() >= budget.MaxFiles().Value() {
		return newSkippedAdmission(
			observation.path,
			sourceSkipReasons["root_file_budget"],
			observedBytes,
			ByteCount{},
			"root admitted-file budget is exhausted",
			usage,
		)
	}
	if observedBytes.Value() > budget.MaxFileBytes().Value() {
		nextUsage, err := advanceAdmissionUsage(usage, ByteCount{})
		if err != nil {
			return nil, usage, err
		}
		return newSkippedAdmission(
			observation.path,
			sourceSkipReasons["oversized"],
			observedBytes,
			budget.MaxFileBytes(),
			"observed source exceeds the per-file byte budget",
			nextUsage,
		)
	}
	remainingBytes := budget.MaxObservedBytes().Value() -
		usage.Bytes().Value()
	if observedBytes.Value() > remainingBytes {
		return newSkippedAdmission(
			observation.path,
			sourceSkipReasons["root_byte_budget"],
			observedBytes,
			budget.MaxObservedBytes(),
			"root admitted-byte budget is exhausted",
			usage,
		)
	}
	content, contentObserved := observation.payload.content()
	if !contentObserved {
		return nil, usage, fmt.Errorf(
			"admissible metadata observation lacks bounded content",
		)
	}
	if !utf8.Valid(content) {
		nextUsage, err := advanceAdmissionUsage(usage, observedBytes)
		if err != nil {
			return nil, usage, err
		}
		return newSkippedAdmission(
			observation.path,
			sourceSkipReasons["invalid_encoding"],
			observedBytes,
			ByteCount{},
			"source is not valid UTF-8",
			nextUsage,
		)
	}
	sum := sha256.Sum256(content)
	source := AdmittedSource{
		path:     observation.path,
		language: observation.language,
		content:  append([]byte(nil), content...),
		digest:   hex.EncodeToString(sum[:]),
	}
	nextUsage, err := advanceAdmissionUsage(usage, observedBytes)
	if err != nil {
		return nil, usage, err
	}
	return sourceAdmitted{source: source}, nextUsage, nil
}

func advanceAdmissionUsage(
	usage AdmissionUsage,
	observedBytes ByteCount,
) (AdmissionUsage, error) {
	if !usage.valid() || !observedBytes.valid() {
		return usage, fmt.Errorf("admission usage increment is invalid")
	}
	nextFiles, err := NewFileCount(usage.Files().Value() + 1)
	if err != nil {
		return usage, err
	}
	nextBytes, err := NewByteCount(
		usage.Bytes().Value() + observedBytes.Value(),
	)
	if err != nil {
		return usage, err
	}
	return AdmissionUsage{files: nextFiles, bytes: nextBytes}, nil
}

func newSkippedAdmission(
	path ProjectPath,
	reason SourceSkipReason,
	observedBytes ByteCount,
	limit ByteCount,
	detail string,
	usage AdmissionUsage,
) (SourceAdmission, AdmissionUsage, error) {
	if !path.valid() ||
		!reason.valid() ||
		!observedBytes.valid() ||
		!limit.valid() ||
		detail == "" {
		return nil, usage, fmt.Errorf("skipped-source inputs are invalid")
	}
	return sourceSkipped{
		path:          path,
		reason:        reason,
		observedBytes: observedBytes,
		limit:         limit,
		detail:        detail,
	}, usage, nil
}

// ObserveSource is the bounded filesystem shell. It reads at most
// MaxFileBytes+1 bytes and returns the exact observation consumed by
// AdmitSource.
func ObserveSource(
	projectRoot string,
	path ProjectPath,
	language SourceLanguage,
	class SourceClass,
	budget IndexBudget,
) (SourceObservation, error) {
	if !path.valid() || !language.valid() || !class.valid() || !budget.valid() {
		return SourceObservation{}, fmt.Errorf("source observation inputs are invalid")
	}
	canonical, err := projectpath.Parse(path.String())
	if err != nil {
		return SourceObservation{}, err
	}
	absolutePath, err := projectpath.ResolveExisting(projectRoot, canonical)
	if err != nil {
		return SourceObservation{}, err
	}
	file, err := os.Open(absolutePath)
	if err != nil {
		return SourceObservation{}, err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		return SourceObservation{}, err
	}
	observedBytes, err := NewByteCount(before.Size())
	if err != nil {
		return SourceObservation{}, err
	}
	if before.Size() > budget.MaxFileBytes().Value() {
		return NewMetadataObservation(
			path,
			language,
			class,
			observedBytes,
		)
	}
	reader := io.LimitReader(file, budget.MaxFileBytes().Value()+1)
	content, err := io.ReadAll(reader)
	if err != nil {
		return SourceObservation{}, err
	}
	after, err := file.Stat()
	if err != nil {
		return SourceObservation{}, err
	}
	if before.Size() != after.Size() ||
		!before.ModTime().Equal(after.ModTime()) {
		return SourceObservation{}, ErrSourceChanged
	}
	return NewContentObservation(path, language, class, content)
}
