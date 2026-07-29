package specmigrationv2

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const FinalCandidatePacketStoreRef = ".haft/spec-migration-v2/packets"

var finalCandidatePacketFilenamePattern = regexp.MustCompile(`^[0-9a-f]{64}\.json$`)

type PacketLocatorDiagnosticCode string

const (
	PacketLocatorStoreNotDirectory  PacketLocatorDiagnosticCode = "store_not_directory"
	PacketLocatorUnsafeTopology     PacketLocatorDiagnosticCode = "unsafe_topology"
	PacketLocatorSymlink            PacketLocatorDiagnosticCode = "symlink"
	PacketLocatorNotRegular         PacketLocatorDiagnosticCode = "not_regular"
	PacketLocatorInvalidFilename    PacketLocatorDiagnosticCode = "invalid_filename"
	PacketLocatorReadFailed         PacketLocatorDiagnosticCode = "read_failed"
	PacketLocatorDecodeFailed       PacketLocatorDiagnosticCode = "decode_failed"
	PacketLocatorDigestNameMismatch PacketLocatorDiagnosticCode = "digest_name_mismatch"
)

type PacketLocatorDiagnostic struct {
	code       PacketLocatorDiagnosticCode
	carrierRef string
	detail     string
}

func (diagnostic PacketLocatorDiagnostic) Code() PacketLocatorDiagnosticCode {
	return diagnostic.code
}

func (diagnostic PacketLocatorDiagnostic) CarrierRef() string {
	return diagnostic.carrierRef
}

func (diagnostic PacketLocatorDiagnostic) Detail() string {
	return diagnostic.detail
}

type InvalidFinalCandidatePacketStoreError struct {
	diagnostics []PacketLocatorDiagnostic
}

func (failure *InvalidFinalCandidatePacketStoreError) Error() string {
	parts := make([]string, 0, len(failure.diagnostics))
	for _, diagnostic := range failure.diagnostics {
		part := fmt.Sprintf(
			"%s [%s]: %s",
			diagnostic.carrierRef,
			diagnostic.code,
			diagnostic.detail,
		)
		parts = append(parts, part)
	}
	return "invalid final-candidate packet store: " + strings.Join(parts, "; ")
}

func (failure *InvalidFinalCandidatePacketStoreError) Diagnostics() []PacketLocatorDiagnostic {
	if failure == nil {
		return []PacketLocatorDiagnostic{}
	}
	return append([]PacketLocatorDiagnostic{}, failure.diagnostics...)
}

type LocatedFinalCandidatePacket struct {
	carrierRef string
	carrier    FinalCandidatePacketCarrier
}

func (candidate LocatedFinalCandidatePacket) CarrierRef() string {
	return candidate.carrierRef
}

func (candidate LocatedFinalCandidatePacket) Carrier() FinalCandidatePacketCarrier {
	return candidate.carrier
}

func (candidate LocatedFinalCandidatePacket) PacketID() MigrationPacketID {
	return candidate.carrier.Packet().ID()
}

func (candidate LocatedFinalCandidatePacket) PacketDigest() PacketDigest {
	return candidate.carrier.PacketDigest()
}

func (candidate LocatedFinalCandidatePacket) CarrierDigest() PacketCarrierDigest {
	return candidate.carrier.CarrierDigest()
}

type FinalCandidatePacketDiscovery interface {
	finalCandidatePacketDiscoveryVariant()
}

type NoFinalCandidatePackets interface {
	FinalCandidatePacketDiscovery
	noFinalCandidatePacketsVariant()
}

type noFinalCandidatePackets struct{}

func (noFinalCandidatePackets) finalCandidatePacketDiscoveryVariant() {}
func (noFinalCandidatePackets) noFinalCandidatePacketsVariant()       {}

type OneFinalCandidatePacket interface {
	FinalCandidatePacketDiscovery
	Candidate() LocatedFinalCandidatePacket
	oneFinalCandidatePacketVariant()
}

type oneFinalCandidatePacket struct {
	candidate LocatedFinalCandidatePacket
}

func (oneFinalCandidatePacket) finalCandidatePacketDiscoveryVariant() {}
func (oneFinalCandidatePacket) oneFinalCandidatePacketVariant()       {}

func (result oneFinalCandidatePacket) Candidate() LocatedFinalCandidatePacket {
	return result.candidate
}

type ManyFinalCandidatePackets interface {
	FinalCandidatePacketDiscovery
	Candidates() []LocatedFinalCandidatePacket
	manyFinalCandidatePacketsVariant()
}

type manyFinalCandidatePackets struct {
	candidates []LocatedFinalCandidatePacket
}

func (manyFinalCandidatePackets) finalCandidatePacketDiscoveryVariant() {}
func (manyFinalCandidatePackets) manyFinalCandidatePacketsVariant()     {}

func (result manyFinalCandidatePackets) Candidates() []LocatedFinalCandidatePacket {
	return append([]LocatedFinalCandidatePacket{}, result.candidates...)
}

// LocateFinalCandidatePackets reads only the canonical project-local packet
// store. It does not select among multiple candidates, inspect applicability,
// admit review, or authorize an effect.
func LocateFinalCandidatePackets(
	root ApplyProjectRoot,
) (FinalCandidatePacketDiscovery, error) {
	if !root.valid() {
		return nil, fmt.Errorf("final-candidate packet locator requires a canonical absolute project root")
	}
	storePath := filepath.Join(
		root.String(),
		filepath.FromSlash(FinalCandidatePacketStoreRef),
	)
	if err := verifyFinalCandidatePacketStoreAncestors(root, storePath); err != nil {
		return nil, err
	}
	storeState, err := inspectFinalCandidatePacketStore(storePath)
	if err != nil {
		return nil, err
	}
	if !storeState.exists {
		return noFinalCandidatePackets{}, nil
	}
	entries, err := os.ReadDir(storePath)
	if err != nil {
		return nil, fmt.Errorf("read final-candidate packet store: %w", err)
	}
	candidates := make([]LocatedFinalCandidatePacket, 0, len(entries))
	diagnostics := make([]PacketLocatorDiagnostic, 0)
	for _, entry := range entries {
		candidate, diagnostic := inspectFinalCandidatePacketEntry(storePath, entry)
		if diagnostic != nil {
			diagnostics = append(diagnostics, *diagnostic)
			continue
		}
		candidates = append(candidates, *candidate)
	}
	if len(diagnostics) > 0 {
		sortPacketLocatorDiagnostics(diagnostics)
		return nil, &InvalidFinalCandidatePacketStoreError{diagnostics: diagnostics}
	}
	sortLocatedFinalCandidatePackets(candidates)
	return classifyFinalCandidatePacketDiscovery(candidates), nil
}

func verifyFinalCandidatePacketStoreAncestors(
	root ApplyProjectRoot,
	storePath string,
) error {
	storeParent := filepath.Dir(storePath)
	if err := verifyConfinedPathComponents(root.String(), storeParent); err != nil {
		diagnostic := newPacketLocatorDiagnostic(
			PacketLocatorUnsafeTopology,
			FinalCandidatePacketStoreRef,
			"packet store ancestors must be real directories inside the project root",
		)
		return invalidPacketStore(diagnostic)
	}
	return nil
}

type finalCandidatePacketStoreState struct {
	exists bool
}

func inspectFinalCandidatePacketStore(
	storePath string,
) (finalCandidatePacketStoreState, error) {
	info, err := os.Lstat(storePath)
	if errors.Is(err, os.ErrNotExist) {
		return finalCandidatePacketStoreState{exists: false}, nil
	}
	if err != nil {
		return finalCandidatePacketStoreState{}, fmt.Errorf("inspect final-candidate packet store: %w", err)
	}
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		diagnostic := newPacketLocatorDiagnostic(
			PacketLocatorSymlink,
			FinalCandidatePacketStoreRef,
			"packet store must be a real directory and cannot be a symlink",
		)
		return finalCandidatePacketStoreState{}, invalidPacketStore(diagnostic)
	}
	if !info.IsDir() {
		diagnostic := newPacketLocatorDiagnostic(
			PacketLocatorStoreNotDirectory,
			FinalCandidatePacketStoreRef,
			"packet store is not a directory",
		)
		return finalCandidatePacketStoreState{}, invalidPacketStore(diagnostic)
	}
	return finalCandidatePacketStoreState{exists: true}, nil
}

func inspectFinalCandidatePacketEntry(
	storePath string,
	entry os.DirEntry,
) (*LocatedFinalCandidatePacket, *PacketLocatorDiagnostic) {
	name := entry.Name()
	carrierRef := filepath.ToSlash(filepath.Join(FinalCandidatePacketStoreRef, name))
	path := filepath.Join(storePath, name)
	info, err := os.Lstat(path)
	if err != nil {
		diagnostic := newPacketLocatorDiagnostic(
			PacketLocatorReadFailed,
			carrierRef,
			err.Error(),
		)
		return nil, &diagnostic
	}
	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		diagnostic := newPacketLocatorDiagnostic(
			PacketLocatorSymlink,
			carrierRef,
			"packet carrier cannot be a symlink",
		)
		return nil, &diagnostic
	}
	if !mode.IsRegular() {
		diagnostic := newPacketLocatorDiagnostic(
			PacketLocatorNotRegular,
			carrierRef,
			"packet carrier is not a regular file",
		)
		return nil, &diagnostic
	}
	if !finalCandidatePacketFilenamePattern.MatchString(name) {
		diagnostic := newPacketLocatorDiagnostic(
			PacketLocatorInvalidFilename,
			carrierRef,
			"packet filename must be <64 lowercase carrier-digest hex>.json",
		)
		return nil, &diagnostic
	}
	bytes, err := readRegularFileNoFollow(path)
	if err != nil {
		diagnostic := newPacketLocatorDiagnostic(
			PacketLocatorReadFailed,
			carrierRef,
			err.Error(),
		)
		return nil, &diagnostic
	}
	carrier, err := DecodePacketCarrier(bytes)
	if err != nil {
		diagnostic := newPacketLocatorDiagnostic(
			PacketLocatorDecodeFailed,
			carrierRef,
			err.Error(),
		)
		return nil, &diagnostic
	}
	expectedName := finalCandidatePacketFilename(carrier)
	if name != expectedName {
		detail := fmt.Sprintf("packet filename is %s; exact carrier digest requires %s", name, expectedName)
		diagnostic := newPacketLocatorDiagnostic(
			PacketLocatorDigestNameMismatch,
			carrierRef,
			detail,
		)
		return nil, &diagnostic
	}
	candidate := LocatedFinalCandidatePacket{
		carrierRef: carrierRef,
		carrier:    carrier,
	}
	return &candidate, nil
}

func classifyFinalCandidatePacketDiscovery(
	candidates []LocatedFinalCandidatePacket,
) FinalCandidatePacketDiscovery {
	switch len(candidates) {
	case 0:
		return noFinalCandidatePackets{}
	case 1:
		return oneFinalCandidatePacket{candidate: candidates[0]}
	default:
		return manyFinalCandidatePackets{
			candidates: append([]LocatedFinalCandidatePacket{}, candidates...),
		}
	}
}

func finalCandidatePacketFilename(carrier FinalCandidatePacketCarrier) string {
	digest := carrier.CarrierDigest().String()
	hex := strings.TrimPrefix(digest, "sha256:")
	return hex + ".json"
}

func sortLocatedFinalCandidatePackets(candidates []LocatedFinalCandidatePacket) {
	sort.Slice(candidates, func(left int, right int) bool {
		leftKey := locatedFinalCandidatePacketSortKey(candidates[left])
		rightKey := locatedFinalCandidatePacketSortKey(candidates[right])
		return leftKey < rightKey
	})
}

func locatedFinalCandidatePacketSortKey(candidate LocatedFinalCandidatePacket) string {
	return candidate.PacketID().String() + "\x00" +
		candidate.CarrierDigest().String() + "\x00" +
		candidate.CarrierRef()
}

func newPacketLocatorDiagnostic(
	code PacketLocatorDiagnosticCode,
	carrierRef string,
	detail string,
) PacketLocatorDiagnostic {
	return PacketLocatorDiagnostic{
		code:       code,
		carrierRef: carrierRef,
		detail:     detail,
	}
}

func invalidPacketStore(
	diagnostic PacketLocatorDiagnostic,
) *InvalidFinalCandidatePacketStoreError {
	return &InvalidFinalCandidatePacketStoreError{
		diagnostics: []PacketLocatorDiagnostic{diagnostic},
	}
}

func sortPacketLocatorDiagnostics(diagnostics []PacketLocatorDiagnostic) {
	sort.Slice(diagnostics, func(left int, right int) bool {
		leftKey := packetLocatorDiagnosticSortKey(diagnostics[left])
		rightKey := packetLocatorDiagnosticSortKey(diagnostics[right])
		return leftKey < rightKey
	})
}

func packetLocatorDiagnosticSortKey(diagnostic PacketLocatorDiagnostic) string {
	return diagnostic.carrierRef + "\x00" + string(diagnostic.code) + "\x00" + diagnostic.detail
}

// EffectJournalCarrierRef delegates to the effect planner's existing path
// derivation so packet discovery and recovery cannot invent a second hash/key
// convention.
func EffectJournalCarrierRef(
	root ApplyProjectRoot,
	packet Packet,
) (string, error) {
	if !root.valid() {
		return "", fmt.Errorf("effect-journal carrier derivation requires a canonical absolute project root")
	}
	if _, err := PacketDigestOf(packet); err != nil {
		return "", fmt.Errorf("effect-journal carrier derivation requires a valid packet: %w", err)
	}
	request := ApplyRequest{
		projectRoot: root,
		analysis: structuralAnalysis{
			packetID: packet.ID(),
		},
	}
	journalPath := effectPaths(request).journal
	relative, err := confinedRelativePath(root.String(), journalPath)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(relative), nil
}
