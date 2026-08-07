// Package projecttypeenvcompatibility compares the logical executable
// semantics of two immutable TypeEnvs without consulting their source
// extension DAGs.
//
// Local owning TypeEnvRefs are normalized so the comparison describes runtime
// behavior rather than composite identity. Source revision, compiler edition,
// coverage, declaration provenance, and availability grounds are deliberately
// outside this comparison; the executable snapshot and Stage edition checks
// authenticate those coordinates separately.
package projecttypeenvcompatibility

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	changeCanonicalDomain = "haft.project-typeenv-executable-change.v1"
	diffCanonicalDomain   = "haft.project-typeenv-executable-diff.v2"
	fingerprintDomain     = "haft.project-typeenv-executable-fingerprint.v1"

	maximumDiffCanonicalBytes = 32 << 20
	maximumDiffChanges        = 1 << 20
	maximumDiffFieldBytes     = 1 << 20
)

// Family identifies one closed TypeEnv semantic surface.
type Family string

const (
	BoundedContextFamily                   Family = "bounded_context"
	KindDefinitionFamily                   Family = "kind_definition"
	EntitySetDefinitionFamily              Family = "entity_set_definition"
	KindSignatureDefinitionFamily          Family = "kind_signature_definition"
	KindClassificationSignatureFamily      Family = "kind_classification_signature_definition"
	RefKindDefinitionFamily                Family = "ref_kind_definition"
	ContextKindAvailabilityFamily          Family = "context_kind_availability"
	SubkindRelationFamily                  Family = "subkind_relation"
	ContextBridgeFamily                    Family = "context_bridge"
	TypedRelationDeclarationFragmentFamily Family = "typed_relation_declaration_fragment"
	// RelationSignatureFamily preserves the source-level API spelling while
	// current compatibility artifacts expose the fragment posture.
	RelationSignatureFamily        = TypedRelationDeclarationFragmentFamily
	RelationSlotFamily      Family = "relation_slot"
	ValueShapeFamily        Family = "value_shape"
	ValueBindingFamily      Family = "value_binding"
	ConstraintFamily        Family = "constraint"
)

const legacyRelationSignatureFamily Family = "relation_signature"

var supportedFamilies = map[Family]struct{}{
	BoundedContextFamily:                   {},
	KindDefinitionFamily:                   {},
	EntitySetDefinitionFamily:              {},
	KindSignatureDefinitionFamily:          {},
	KindClassificationSignatureFamily:      {},
	RefKindDefinitionFamily:                {},
	ContextKindAvailabilityFamily:          {},
	SubkindRelationFamily:                  {},
	ContextBridgeFamily:                    {},
	TypedRelationDeclarationFragmentFamily: {},
	legacyRelationSignatureFamily:          {},
	RelationSlotFamily:                     {},
	ValueShapeFamily:                       {},
	ValueBindingFamily:                     {},
	ConstraintFamily:                       {},
}

func (family Family) valid() bool {
	_, exists := supportedFamilies[family]
	return exists
}

// Change is a closed immutable added/changed/removed semantic delta.
type Change interface {
	Family() Family
	Key() string
	Kind() typedmemory.CompatibilityChangeKind
	BeforeDigest() (typedmemory.SHA256Digest, bool)
	AfterDigest() (typedmemory.SHA256Digest, bool)
	CanonicalBytes() []byte
	changeVariant()
}

// AddedChange records one newly exposed executable semantic row.
type AddedChange struct {
	family Family
	key    string
	after  typedmemory.SHA256Digest
}

func (change AddedChange) Family() Family { return change.family }

func (change AddedChange) Key() string { return change.key }

func (AddedChange) Kind() typedmemory.CompatibilityChangeKind {
	return typedmemory.CompatibilityAdded
}

func (AddedChange) BeforeDigest() (typedmemory.SHA256Digest, bool) {
	return typedmemory.SHA256Digest{}, false
}

func (change AddedChange) AfterDigest() (typedmemory.SHA256Digest, bool) {
	return change.after, true
}

func (change AddedChange) CanonicalBytes() []byte {
	writer := newCanonicalWriter(changeCanonicalDomain)
	writer.addString(change.family.String())
	writer.addString(change.key)
	writer.addString(change.Kind().String())
	writer.addString(change.after.String())
	return writer.bytes()
}

func (AddedChange) changeVariant() {}

// ChangedChange records exact prior and current fingerprints for one stable
// semantic coordinate.
type ChangedChange struct {
	family Family
	key    string
	before typedmemory.SHA256Digest
	after  typedmemory.SHA256Digest
}

func (change ChangedChange) Family() Family { return change.family }

func (change ChangedChange) Key() string { return change.key }

func (ChangedChange) Kind() typedmemory.CompatibilityChangeKind {
	return typedmemory.CompatibilityChanged
}

func (change ChangedChange) BeforeDigest() (typedmemory.SHA256Digest, bool) {
	return change.before, true
}

func (change ChangedChange) AfterDigest() (typedmemory.SHA256Digest, bool) {
	return change.after, true
}

func (change ChangedChange) CanonicalBytes() []byte {
	writer := newCanonicalWriter(changeCanonicalDomain)
	writer.addString(change.family.String())
	writer.addString(change.key)
	writer.addString(change.Kind().String())
	writer.addString(change.before.String())
	writer.addString(change.after.String())
	return writer.bytes()
}

func (ChangedChange) changeVariant() {}

// RemovedChange records one executable semantic row no longer exposed.
type RemovedChange struct {
	family Family
	key    string
	before typedmemory.SHA256Digest
}

func (change RemovedChange) Family() Family { return change.family }

func (change RemovedChange) Key() string { return change.key }

func (RemovedChange) Kind() typedmemory.CompatibilityChangeKind {
	return typedmemory.CompatibilityRemoved
}

func (change RemovedChange) BeforeDigest() (typedmemory.SHA256Digest, bool) {
	return change.before, true
}

func (RemovedChange) AfterDigest() (typedmemory.SHA256Digest, bool) {
	return typedmemory.SHA256Digest{}, false
}

func (change RemovedChange) CanonicalBytes() []byte {
	writer := newCanonicalWriter(changeCanonicalDomain)
	writer.addString(change.family.String())
	writer.addString(change.key)
	writer.addString(change.Kind().String())
	writer.addString(change.before.String())
	return writer.bytes()
}

func (RemovedChange) changeVariant() {}

// Diff is the immutable, canonical transition comparison from Base to Target.
// Both exact TypeEnv coordinates and every sorted semantic change belong to
// its identity. It is therefore sufficient for a Stage to retain and replay
// the complete comparison rather than a lossy symbol-only projection.
type Diff struct {
	base    typedmemory.TypeEnvRef
	target  typedmemory.TypeEnvRef
	changes []Change
	digest  typedmemory.SHA256Digest
}

// Compare derives a complete executable TypeEnv diff from the two immutable
// environments. It never accepts a caller-supplied change list.
func Compare(
	previous typedmemory.TypeEnv,
	current typedmemory.TypeEnv,
) (Diff, error) {
	before, err := projectTypeEnv(previous)
	if err != nil {
		return Diff{}, fmt.Errorf("project previous executable TypeEnv: %w", err)
	}
	after, err := projectTypeEnv(current)
	if err != nil {
		return Diff{}, fmt.Errorf("project current executable TypeEnv: %w", err)
	}
	changes := compareSemanticEntries(before, after)
	diff := Diff{
		base:    previous.Ref(),
		target:  current.Ref(),
		changes: changes,
	}
	digest, err := digestBytes(diff.CanonicalBytes())
	if err != nil {
		return Diff{}, fmt.Errorf("derive executable TypeEnv diff digest: %w", err)
	}
	diff.digest = digest
	if err := diff.Verify(); err != nil {
		return Diff{}, fmt.Errorf("verify executable TypeEnv diff: %w", err)
	}
	return diff, nil
}

func (diff Diff) Base() typedmemory.TypeEnvRef { return diff.base }

func (diff Diff) Target() typedmemory.TypeEnvRef { return diff.target }

func (diff Diff) Changes() []Change {
	return append([]Change(nil), diff.changes...)
}

func (diff Diff) Empty() bool { return len(diff.changes) == 0 }

func (diff Diff) CanonicalBytes() []byte {
	writer := newCanonicalWriter(diffCanonicalDomain)
	writer.addString(diff.base.String())
	writer.addString(diff.target.String())
	writer.addUint64(uint64(len(diff.changes)))
	for _, change := range diff.changes {
		writer.addBytes(change.CanonicalBytes())
	}
	return writer.bytes()
}

func (diff Diff) Digest() typedmemory.SHA256Digest {
	return diff.digest
}

func (diff Diff) Verify() error {
	base, err := typedmemory.ParseTypeEnvRef(diff.base.String())
	if err != nil || base != diff.base {
		return fmt.Errorf("executable TypeEnv diff base is invalid")
	}
	target, err := typedmemory.ParseTypeEnvRef(diff.target.String())
	if err != nil || target != diff.target {
		return fmt.Errorf("executable TypeEnv diff target is invalid")
	}
	if len(diff.changes) > maximumDiffChanges {
		return fmt.Errorf("executable TypeEnv diff exceeds %d changes", maximumDiffChanges)
	}
	for index, change := range diff.changes {
		if change == nil {
			return fmt.Errorf("executable TypeEnv diff change %d is missing", index)
		}
		decoded, decodeErr := decodeChange(change.CanonicalBytes())
		if decodeErr != nil {
			return fmt.Errorf(
				"verify executable TypeEnv diff change %d: %w",
				index,
				decodeErr,
			)
		}
		if !bytes.Equal(decoded.CanonicalBytes(), change.CanonicalBytes()) {
			return fmt.Errorf("executable TypeEnv diff change %d is not exact", index)
		}
	}
	if err := verifyCanonicalChangeOrder(diff.changes); err != nil {
		return err
	}
	canonical := diff.CanonicalBytes()
	if len(canonical) > maximumDiffCanonicalBytes {
		return fmt.Errorf(
			"executable TypeEnv diff exceeds %d bytes",
			maximumDiffCanonicalBytes,
		)
	}
	digest, err := digestBytes(canonical)
	if err != nil {
		return fmt.Errorf("derive executable TypeEnv diff digest: %w", err)
	}
	if digest != diff.digest {
		return fmt.Errorf("executable TypeEnv diff digest mismatch")
	}
	return nil
}

// DecodeDiff strictly restores one canonical compatibility diff. It does not
// infer compatibility or accept a caller-supplied digest; Compare remains the
// only producer from executable TypeEnvs.
func DecodeDiff(canonical []byte) (Diff, error) {
	if len(canonical) == 0 {
		return Diff{}, fmt.Errorf("executable TypeEnv diff is empty")
	}
	if len(canonical) > maximumDiffCanonicalBytes {
		return Diff{}, fmt.Errorf(
			"executable TypeEnv diff exceeds %d bytes",
			maximumDiffCanonicalBytes,
		)
	}
	reader := newCanonicalReader(canonical)
	domain, err := reader.readString("diff domain")
	if err != nil {
		return Diff{}, err
	}
	if domain != diffCanonicalDomain {
		return Diff{}, fmt.Errorf("executable TypeEnv diff domain is invalid")
	}
	baseText, err := reader.readString("base TypeEnv")
	if err != nil {
		return Diff{}, err
	}
	base, err := typedmemory.ParseTypeEnvRef(baseText)
	if err != nil {
		return Diff{}, fmt.Errorf("decode base TypeEnv: %w", err)
	}
	targetText, err := reader.readString("target TypeEnv")
	if err != nil {
		return Diff{}, err
	}
	target, err := typedmemory.ParseTypeEnvRef(targetText)
	if err != nil {
		return Diff{}, fmt.Errorf("decode target TypeEnv: %w", err)
	}
	count, err := reader.readCount("semantic changes", maximumDiffChanges)
	if err != nil {
		return Diff{}, err
	}
	changes := make([]Change, 0, count)
	for index := 0; index < count; index++ {
		changeBytes, readErr := reader.readBytes("semantic change")
		if readErr != nil {
			return Diff{}, fmt.Errorf("decode semantic change %d: %w", index, readErr)
		}
		change, decodeErr := decodeChange(changeBytes)
		if decodeErr != nil {
			return Diff{}, fmt.Errorf("decode semantic change %d: %w", index, decodeErr)
		}
		changes = append(changes, change)
	}
	if reader.remaining() != 0 {
		return Diff{}, fmt.Errorf("executable TypeEnv diff has trailing bytes")
	}
	if err := verifyCanonicalChangeOrder(changes); err != nil {
		return Diff{}, err
	}
	diff := Diff{
		base:    base,
		target:  target,
		changes: changes,
	}
	reencoded := diff.CanonicalBytes()
	if !bytes.Equal(reencoded, canonical) {
		return Diff{}, fmt.Errorf("executable TypeEnv diff is not canonical")
	}
	digest, err := digestBytes(reencoded)
	if err != nil {
		return Diff{}, fmt.Errorf("derive decoded executable TypeEnv diff digest: %w", err)
	}
	diff.digest = digest
	if err := diff.Verify(); err != nil {
		return Diff{}, err
	}
	return diff, nil
}

func decodeChange(canonical []byte) (Change, error) {
	if len(canonical) == 0 {
		return nil, fmt.Errorf("executable TypeEnv change is empty")
	}
	reader := newCanonicalReader(canonical)
	domain, err := reader.readString("change domain")
	if err != nil {
		return nil, err
	}
	if domain != changeCanonicalDomain {
		return nil, fmt.Errorf("executable TypeEnv change domain is invalid")
	}
	familyText, err := reader.readString("change family")
	if err != nil {
		return nil, err
	}
	family, err := parseFamily(familyText)
	if err != nil {
		return nil, err
	}
	key, err := reader.readString("change key")
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, fmt.Errorf("change key is required")
	}
	kindText, err := reader.readString("change kind")
	if err != nil {
		return nil, err
	}
	kind, err := parseChangeKind(kindText)
	if err != nil {
		return nil, err
	}
	var change Change
	switch kind {
	case typedmemory.CompatibilityAdded:
		after, readErr := reader.readDigest("after digest")
		if readErr != nil {
			return nil, readErr
		}
		change = AddedChange{family: family, key: key, after: after}
	case typedmemory.CompatibilityChanged:
		before, readErr := reader.readDigest("before digest")
		if readErr != nil {
			return nil, readErr
		}
		after, readErr := reader.readDigest("after digest")
		if readErr != nil {
			return nil, readErr
		}
		if before == after {
			return nil, fmt.Errorf("changed semantic row has equal before and after digests")
		}
		change = ChangedChange{
			family: family,
			key:    key,
			before: before,
			after:  after,
		}
	case typedmemory.CompatibilityRemoved:
		before, readErr := reader.readDigest("before digest")
		if readErr != nil {
			return nil, readErr
		}
		change = RemovedChange{family: family, key: key, before: before}
	default:
		return nil, fmt.Errorf("change kind %q is unsupported", kindText)
	}
	if reader.remaining() != 0 {
		return nil, fmt.Errorf("executable TypeEnv change has trailing bytes")
	}
	if !bytes.Equal(change.CanonicalBytes(), canonical) {
		return nil, fmt.Errorf("executable TypeEnv change is not canonical")
	}
	return change, nil
}

func parseFamily(raw string) (Family, error) {
	family := Family(raw)
	if !family.valid() {
		return "", fmt.Errorf("unsupported TypeEnv semantic family %q", raw)
	}
	return family, nil
}

func parseChangeKind(raw string) (typedmemory.CompatibilityChangeKind, error) {
	switch raw {
	case typedmemory.CompatibilityAdded.String():
		return typedmemory.CompatibilityAdded, nil
	case typedmemory.CompatibilityChanged.String():
		return typedmemory.CompatibilityChanged, nil
	case typedmemory.CompatibilityRemoved.String():
		return typedmemory.CompatibilityRemoved, nil
	default:
		return 0, fmt.Errorf("unsupported compatibility change kind %q", raw)
	}
}

func verifyCanonicalChangeOrder(changes []Change) error {
	for index := 1; index < len(changes); index++ {
		previous := changes[index-1]
		current := changes[index]
		comparison := bytes.Compare(
			[]byte(previous.Family().String()),
			[]byte(current.Family().String()),
		)
		if comparison > 0 {
			return fmt.Errorf("semantic changes are not canonically ordered")
		}
		if comparison == 0 && previous.Key() >= current.Key() {
			return fmt.Errorf(
				"semantic changes contain duplicate or non-canonical coordinate %s/%s",
				current.Family(),
				current.Key(),
			)
		}
	}
	return nil
}

type semanticEntry struct {
	family      Family
	key         string
	material    []byte
	fingerprint typedmemory.SHA256Digest
}

func newSemanticEntry(
	family Family,
	key string,
	material []byte,
) (semanticEntry, error) {
	if !family.valid() {
		return semanticEntry{}, fmt.Errorf("unsupported TypeEnv semantic family %q", family)
	}
	if key == "" {
		return semanticEntry{}, fmt.Errorf("%s semantic key is required", family)
	}
	writer := newCanonicalWriter(fingerprintDomain)
	writer.addString(family.String())
	writer.addString(key)
	writer.addBytes(material)
	fingerprint, err := digestBytes(writer.bytes())
	if err != nil {
		return semanticEntry{}, err
	}
	return semanticEntry{
		family:      family,
		key:         key,
		material:    append([]byte(nil), material...),
		fingerprint: fingerprint,
	}, nil
}

func compareSemanticEntries(
	before []semanticEntry,
	after []semanticEntry,
) []Change {
	changes := make([]Change, 0)
	beforeIndex := 0
	afterIndex := 0
	for beforeIndex < len(before) || afterIndex < len(after) {
		comparison := compareEntryPositions(before, beforeIndex, after, afterIndex)
		if comparison < 0 {
			entry := before[beforeIndex]
			changes = append(changes, RemovedChange{
				family: entry.family,
				key:    entry.key,
				before: entry.fingerprint,
			})
			beforeIndex++
			continue
		}
		if comparison > 0 {
			entry := after[afterIndex]
			changes = append(changes, AddedChange{
				family: entry.family,
				key:    entry.key,
				after:  entry.fingerprint,
			})
			afterIndex++
			continue
		}
		beforeEntry := before[beforeIndex]
		afterEntry := after[afterIndex]
		if beforeEntry.fingerprint != afterEntry.fingerprint {
			changes = append(changes, ChangedChange{
				family: beforeEntry.family,
				key:    beforeEntry.key,
				before: beforeEntry.fingerprint,
				after:  afterEntry.fingerprint,
			})
		}
		beforeIndex++
		afterIndex++
	}
	return changes
}

func compareEntryPositions(
	before []semanticEntry,
	beforeIndex int,
	after []semanticEntry,
	afterIndex int,
) int {
	if beforeIndex >= len(before) {
		return 1
	}
	if afterIndex >= len(after) {
		return -1
	}
	return compareSemanticEntryCoordinates(before[beforeIndex], after[afterIndex])
}

func compareSemanticEntryCoordinates(left, right semanticEntry) int {
	familyComparison := bytes.Compare(
		[]byte(left.family.String()),
		[]byte(right.family.String()),
	)
	if familyComparison != 0 {
		return familyComparison
	}
	return bytes.Compare([]byte(left.key), []byte(right.key))
}

func sortSemanticEntries(entries []semanticEntry) error {
	sort.Slice(entries, func(left, right int) bool {
		return compareSemanticEntryCoordinates(entries[left], entries[right]) < 0
	})
	for index := 1; index < len(entries); index++ {
		if compareSemanticEntryCoordinates(entries[index-1], entries[index]) == 0 {
			return fmt.Errorf(
				"duplicate %s semantic coordinate %q",
				entries[index].family,
				entries[index].key,
			)
		}
	}
	return nil
}

func digestBytes(canonical []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	encoded := hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest("sha256:" + encoded)
}

type canonicalWriter struct {
	buffer bytes.Buffer
}

func newCanonicalWriter(domain string) *canonicalWriter {
	writer := &canonicalWriter{}
	writer.addString(domain)
	return writer
}

func (writer *canonicalWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *canonicalWriter) addBytes(value []byte) {
	length := make([]byte, 8)
	binary.BigEndian.PutUint64(length, uint64(len(value)))
	_, _ = writer.buffer.Write(length)
	_, _ = writer.buffer.Write(value)
}

func (writer *canonicalWriter) addUint64(value uint64) {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, value)
	writer.addBytes(encoded)
}

func (writer *canonicalWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type canonicalReader struct {
	value  []byte
	offset int
}

func newCanonicalReader(value []byte) *canonicalReader {
	return &canonicalReader{value: append([]byte(nil), value...)}
}

func (reader *canonicalReader) readString(label string) (string, error) {
	value, err := reader.readBytes(label)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", fmt.Errorf("%s is not valid UTF-8", label)
	}
	return string(value), nil
}

func (reader *canonicalReader) readDigest(
	label string,
) (typedmemory.SHA256Digest, error) {
	text, err := reader.readString(label)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(text)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf("%s: %w", label, err)
	}
	return digest, nil
}

func (reader *canonicalReader) readCount(label string, maximum int) (int, error) {
	value, err := reader.readBytes(label)
	if err != nil {
		return 0, err
	}
	if len(value) != 8 {
		return 0, fmt.Errorf("%s must be an encoded uint64", label)
	}
	count := binary.BigEndian.Uint64(value)
	if maximum < 0 {
		return 0, fmt.Errorf("%s maximum must be nonnegative", label)
	}
	maximumValue := uint64(maximum) // #nosec G115 -- maximum is nonnegative above.
	if count > maximumValue {
		return 0, fmt.Errorf("%s exceeds %d", label, maximum)
	}
	return int(count), nil // #nosec G115 -- count is bounded by the nonnegative int maximum above.
}

func (reader *canonicalReader) readBytes(label string) ([]byte, error) {
	if reader.remaining() < 8 {
		return nil, fmt.Errorf("%s length is truncated", label)
	}
	length := binary.BigEndian.Uint64(reader.value[reader.offset : reader.offset+8])
	reader.offset += 8
	if length > uint64(maximumDiffFieldBytes) {
		return nil, fmt.Errorf("%s exceeds %d bytes", label, maximumDiffFieldBytes)
	}
	remaining := reader.remaining()
	if remaining < 0 {
		return nil, fmt.Errorf("%s reader offset is invalid", label)
	}
	remainingValue := uint64(remaining) // #nosec G115 -- remaining is nonnegative above.
	if length > remainingValue {
		return nil, fmt.Errorf("%s bytes are truncated", label)
	}
	end := reader.offset + int(length)
	value := append([]byte(nil), reader.value[reader.offset:end]...)
	reader.offset = end
	return value, nil
}

func (reader *canonicalReader) remaining() int {
	return len(reader.value) - reader.offset
}

func (family Family) String() string { return string(family) }
