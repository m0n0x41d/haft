package specmigrationv2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"sort"
	"strconv"
	"strings"
)

const packetDigestDomain = "haft.spec-migration-v2.packet/v2"

type packetDigestWriter struct {
	hash hash.Hash
}

func newPacketDigestWriter() packetDigestWriter {
	writer := packetDigestWriter{hash: sha256.New()}
	writer.add(packetDigestDomain)
	return writer
}

func (writer packetDigestWriter) add(value string) {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	_, _ = writer.hash.Write(size[:])
	_, _ = writer.hash.Write([]byte(value))
}

func (writer packetDigestWriter) digest() PacketDigest {
	encoded := hex.EncodeToString(writer.hash.Sum(nil))
	value := SHA256{value: "sha256:" + encoded}
	return PacketDigest{value: value}
}

func PacketDigestOf(packet Packet) (PacketDigest, error) {
	if !packet.id.valid() ||
		!packet.source.valid() ||
		!packet.target.valid() ||
		!packet.lineagePolicy.valid() {
		return PacketDigest{}, fmt.Errorf("cannot digest an invalid migration packet")
	}
	writer := newPacketDigestWriter()
	writer.add(packet.id.String())
	writer.add(strconv.FormatUint(uint64(packet.schemaVersion), 10))
	addSourceManifestDigest(writer, packet.source)
	addTargetManifestDigest(writer, packet.target)
	addOutsideRegistryDigest(writer, packet.outsideRegistry)
	if err := addSourceDispositionsDigest(writer, packet.sourceDispositions); err != nil {
		return PacketDigest{}, err
	}
	lineageDigest, err := LineagePolicyDigestOf(packet.lineagePolicy)
	if err != nil {
		return PacketDigest{}, err
	}
	writer.add(lineageDigest.String())
	return writer.digest(), nil
}

func addSourceManifestDigest(writer packetDigestWriter, manifest SourceManifest) {
	writer.add(manifest.carrier.String())
	writer.add(manifest.digest.String())
	writer.add(strconv.FormatUint(manifest.byteLength.Value(), 10))
	writer.add(manifest.archive.carrier.String())
	writer.add(manifest.archive.sourceDigest.String())
	addSourceProvenanceDigest(writer, manifest.provenance)
	sections := append([]SourceSection{}, manifest.sections...)
	sort.Slice(sections, func(left, right int) bool {
		leftKey := sourceSectionDigestSortKey(sections[left])
		rightKey := sourceSectionDigestSortKey(sections[right])
		return leftKey < rightKey
	})
	writer.add(strconv.Itoa(len(sections)))
	for _, section := range sections {
		writer.add(section.id.String())
		addSpanDigest(writer, section.span)
	}
}

func addSourceProvenanceDigest(
	writer packetDigestWriter,
	provenance DesignatedSourceProvenance,
) {
	switch origin := provenance.origin.(type) {
	case RepositoryEdition:
		writer.add("repository_edition")
		addRepositoryEditionDigest(writer, origin)
	case WorkingTreeEdition:
		writer.add("working_tree_edition")
		addRepositoryEditionDigest(writer, origin.parent)
		writer.add(origin.designatedDigest.String())
		writer.add(string(origin.delta.format))
		writer.add(origin.delta.digest.String())
	default:
		writer.add("unknown_source_edition")
	}
	writer.add(provenance.resolutionRecord.ref.String())
	writer.add(provenance.resolutionRecord.digest.String())
}

func addRepositoryEditionDigest(
	writer packetDigestWriter,
	edition RepositoryEdition,
) {
	writer.add(edition.projectRoot.String())
	writer.add(edition.commitOID.String())
	writer.add(edition.carrier.String())
	writer.add(edition.bytesDigest.String())
}

func addTargetManifestDigest(writer packetDigestWriter, manifest TargetManifest) {
	writer.add(manifest.carrier.String())
	writer.add(manifest.digest.String())
	writer.add(strconv.FormatUint(manifest.byteLength.Value(), 10))
}

func addOutsideRegistryDigest(writer packetDigestWriter, registry OutsideCarrierRegistry) {
	values := append([]OutsideCarrierRegistration{}, registry.values...)
	sort.Slice(values, func(left, right int) bool {
		return values[left].id.String() < values[right].id.String()
	})
	writer.add(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.add(value.id.String())
		writer.add(value.carrier.String())
		writer.add(value.digest.String())
	}
}

func addSourceDispositionsDigest(
	writer packetDigestWriter,
	values []SourceDisposition,
) error {
	dispositions := append([]SourceDisposition{}, values...)
	sort.Slice(dispositions, func(left, right int) bool {
		leftKey := sourceDispositionDigestSortKey(dispositions[left])
		rightKey := sourceDispositionDigestSortKey(dispositions[right])
		return leftKey < rightKey
	})
	writer.add(strconv.Itoa(len(dispositions)))
	for _, disposition := range dispositions {
		writer.add(disposition.source.String())
		if err := addDispositionDigest(writer, disposition.disposition); err != nil {
			return err
		}
	}
	return nil
}

func sourceSectionDigestSortKey(section SourceSection) string {
	span := section.span
	return section.id.String() + "\x00" +
		fmt.Sprintf("%020d\x00%020d\x00", span.Start(), span.Length().Value()) +
		span.Digest().String()
}

func sourceDispositionDigestSortKey(disposition SourceDisposition) string {
	return disposition.source.String() + "\x00" + dispositionDigestSortKey(disposition.disposition)
}

func dispositionDigestSortKey(disposition Disposition) string {
	switch value := disposition.(type) {
	case MapOne:
		claims := value.targetClaims.Values()
		parts := make([]string, 0, len(claims))
		for _, claim := range claims {
			parts = append(parts, claim.String())
		}
		sort.Strings(parts)
		return "MapOne\x00" + strings.Join(parts, "\x00")
	case SplitOneToMany:
		parts := make([]string, 0, len(value.branches))
		for _, branch := range value.branches {
			span := branch.fragment
			part := fmt.Sprintf("%020d\x00%020d\x00", span.Start(), span.Length().Value())
			part += span.Digest().String() + "\x00"
			part += branchDispositionDigestSortKey(branch.disposition)
			parts = append(parts, part)
		}
		sort.Strings(parts)
		return "SplitOneToMany\x00" + strings.Join(parts, "\x01")
	case RetireHistory:
		return "RetireHistory\x00" + value.reason
	case OutsidePSS:
		carrierIDs := value.carriers.Values()
		parts := make([]string, 0, len(carrierIDs))
		for _, carrierID := range carrierIDs {
			parts = append(parts, carrierID.String())
		}
		sort.Strings(parts)
		return "OutsidePSS\x00" + value.meaning + "\x00" + strings.Join(parts, "\x00")
	default:
		return "UnknownDisposition"
	}
}

func branchDispositionDigestSortKey(disposition BranchDisposition) string {
	switch value := disposition.(type) {
	case MapOne:
		return dispositionDigestSortKey(value)
	case RetireHistory:
		return dispositionDigestSortKey(value)
	case OutsidePSS:
		return dispositionDigestSortKey(value)
	default:
		return "UnknownBranchDisposition"
	}
}

func addDispositionDigest(writer packetDigestWriter, disposition Disposition) error {
	switch value := disposition.(type) {
	case MapOne:
		writer.add("MapOne")
		addTargetClaimsDigest(writer, value.targetClaims)
		return nil
	case SplitOneToMany:
		writer.add("SplitOneToMany")
		return addSplitDigest(writer, value)
	case RetireHistory:
		writer.add("RetireHistory")
		writer.add(value.reason)
		return nil
	case OutsidePSS:
		writer.add("OutsidePSS")
		addOutsidePSSDigest(writer, value)
		return nil
	default:
		return fmt.Errorf("cannot digest unknown disposition variant")
	}
}

func addBranchDispositionDigest(
	writer packetDigestWriter,
	disposition BranchDisposition,
) error {
	switch value := disposition.(type) {
	case MapOne:
		return addDispositionDigest(writer, value)
	case RetireHistory:
		return addDispositionDigest(writer, value)
	case OutsidePSS:
		return addDispositionDigest(writer, value)
	default:
		return fmt.Errorf("cannot digest unknown split-branch disposition variant")
	}
}

func addSplitDigest(writer packetDigestWriter, split SplitOneToMany) error {
	branches := append([]SplitBranch{}, split.branches...)
	sort.Slice(branches, func(left, right int) bool {
		leftSpan := branches[left].fragment
		rightSpan := branches[right].fragment
		if leftSpan.Start() == rightSpan.Start() {
			return leftSpan.End() < rightSpan.End()
		}
		return leftSpan.Start() < rightSpan.Start()
	})
	writer.add(strconv.Itoa(len(branches)))
	for _, branch := range branches {
		addSpanDigest(writer, branch.fragment)
		if err := addBranchDispositionDigest(writer, branch.disposition); err != nil {
			return err
		}
	}
	return nil
}

func addTargetClaimsDigest(writer packetDigestWriter, claims TargetClaimSet) {
	values := append([]TargetAtomicClaimID{}, claims.values...)
	sort.Slice(values, func(left, right int) bool {
		return values[left].String() < values[right].String()
	})
	writer.add(claims.section.String())
	writer.add(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.add(value.String())
	}
}

func addOutsidePSSDigest(writer packetDigestWriter, outside OutsidePSS) {
	writer.add(outside.meaning)
	values := append([]OutsideCarrierID{}, outside.carriers.values...)
	sort.Slice(values, func(left, right int) bool {
		return values[left].String() < values[right].String()
	})
	writer.add(strconv.Itoa(len(values)))
	for _, value := range values {
		writer.add(value.String())
	}
}

func addSpanDigest(writer packetDigestWriter, span ExactByteSpan) {
	writer.add(strconv.FormatUint(span.Start(), 10))
	writer.add(strconv.FormatUint(span.Length().Value(), 10))
	writer.add(span.Digest().String())
}
