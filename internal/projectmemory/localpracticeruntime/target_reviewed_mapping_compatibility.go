package localpracticeruntime

import (
	"fmt"

	"github.com/m0n0x41d/haft/internal/recordmapping"
	"github.com/m0n0x41d/haft/internal/recordmembershipregistration"
)

// mappingCoordinateV1 is one exact mapping-manifest/adapter pair from the
// shipped 1.0 producer family.
type mappingCoordinateV1 struct {
	manifest string
	adapter  string
}

var projectRecordShippedV1MappingCoordinates = []mappingCoordinateV1{
	{
		manifest: "mapping-manifest:18:haft.evidence-work5:1.0.0sha256:bb010a20f5c691e1a615b8a7e891bef1a4087e89695099b124df2a4e8b22069e",
		adapter:  "haft-evidence-work-adapter/1.0.0",
	},
	{
		manifest: "mapping-manifest:20:haft.note-at-concern5:1.0.0sha256:c22309ff58a1f1be7474841f5232d43ec0024f423f8fe22336330b2700ba6f53",
		adapter:  "haft-note-adapter/1.0.0",
	},
	{
		manifest: "mapping-manifest:25:haft.portfolio-comparison5:1.0.0sha256:28d0f994ab80bb8ff70d63a9361d8d737da1d177a334edf5def8c31ad3ac68a1",
		adapter:  "haft-portfolio-comparison-adapter/1.0.0",
	},
	{
		manifest: "mapping-manifest:28:haft.problem-card-at-concern5:1.0.0sha256:0ea0ed8ac6340eb7a3c4857480a2494423df1763dad403430671f226849ab3a7",
		adapter:  "haft-problem-card-adapter/1.0.0",
	},
	{
		manifest: "mapping-manifest:28:haft.spec-section-at-concern5:1.0.0sha256:92bbceedf8989609775f279fb37aa641de050dd4335ad4ba1efab47d6fefc412",
		adapter:  "haft-spec-section-adapter/1.0.0",
	},
	{
		manifest: "mapping-manifest:31:haft.decision-choice-at-concern5:1.1.0sha256:85de30a8ab311ae9254131f2b2c08b39744a1512879a7233aaf8c3d84fc82545",
		adapter:  "haft-decision-record-adapter/1.1.0",
	},
	{
		manifest: "mapping-manifest:34:haft.solution-portfolio-at-concern5:1.0.0sha256:eb3b9729e37f93cd93608611910e41bac66d98462b318c20b09b54a7d93d8b65",
		adapter:  "haft-solution-portfolio-adapter/1.0.0",
	},
}

var codeAnchorShippedV1MappingCoordinates = []mappingCoordinateV1{
	{
		manifest: "mapping-manifest:16:haft.code-anchor5:1.0.0sha256:a4e5dad8db3dd94922f45585d23b3f3acb6790689ac06ceac2db00636d2a9a7d",
		adapter:  "haft-code-anchor-adapter/1.0.0",
	},
}

// The target-reviewed compatibility catalogs are explicit target-1.1 policy,
// not facts inferred from a predecessor policy or a project database. Their
// acceptance is target-wide: registration does not distinguish a durable
// historical source from a request-scoped overlay. Current public producer
// adapters remain on their own 2.0 mapping coordinates.
var projectRecordTargetReviewedCompatibilityCoordinatesV1 = []mappingCoordinateV1{
	projectRecordShippedV1MappingCoordinates[0],
	projectRecordShippedV1MappingCoordinates[1],
	projectRecordShippedV1MappingCoordinates[2],
	projectRecordShippedV1MappingCoordinates[3],
	projectRecordShippedV1MappingCoordinates[4],
	projectRecordShippedV1MappingCoordinates[5],
	projectRecordShippedV1MappingCoordinates[6],
}

var codeAnchorTargetReviewedCompatibilityCoordinatesV1 = []mappingCoordinateV1{
	codeAnchorShippedV1MappingCoordinates[0],
}

// ProjectRecordTargetReviewedCompatibilityMappingsV1 returns the exact shipped
// 1.0 producer coordinates deliberately supported by the target-1.1
// ProjectRecord registration policy.
func ProjectRecordTargetReviewedCompatibilityMappingsV1() (
	[]recordmembershipregistration.AcceptedMapping,
	error,
) {
	return decodeMappingCoordinatesV1(
		projectRecordTargetReviewedCompatibilityCoordinatesV1,
	)
}

// CodeAnchorTargetReviewedCompatibilityMappingsV1 returns the exact shipped
// 1.0 task-adapter coordinate deliberately supported by the target-1.1
// CodeAnchor registration policy. The unchanged carrier-family coordinate is
// supplied separately by both editions.
func CodeAnchorTargetReviewedCompatibilityMappingsV1() (
	[]recordmembershipregistration.AcceptedMapping,
	error,
) {
	return decodeMappingCoordinatesV1(
		codeAnchorTargetReviewedCompatibilityCoordinatesV1,
	)
}

func projectRecordShippedV1AcceptedMappings() (
	[]recordmembershipregistration.AcceptedMapping,
	error,
) {
	return decodeMappingCoordinatesV1(projectRecordShippedV1MappingCoordinates)
}

func codeAnchorShippedV1AcceptedMappings() (
	[]recordmembershipregistration.AcceptedMapping,
	error,
) {
	return decodeMappingCoordinatesV1(codeAnchorShippedV1MappingCoordinates)
}

func decodeMappingCoordinatesV1(
	coordinates []mappingCoordinateV1,
) ([]recordmembershipregistration.AcceptedMapping, error) {
	result := make(
		[]recordmembershipregistration.AcceptedMapping,
		0,
		len(coordinates),
	)
	for index, coordinate := range coordinates {
		manifest, err := recordmapping.ParseMappingManifestRef(coordinate.manifest)
		if err != nil {
			return nil, fmt.Errorf(
				"decode mapping compatibility %d manifest: %w",
				index,
				err,
			)
		}
		adapter, err := recordmapping.NewAdapterVersion(coordinate.adapter)
		if err != nil {
			return nil, fmt.Errorf(
				"decode mapping compatibility %d adapter: %w",
				index,
				err,
			)
		}
		mapping, err := recordmembershipregistration.NewAcceptedMapping(
			recordmembershipregistration.AcceptedMappingInput{
				Manifest: manifest,
				Adapter:  adapter,
			},
		)
		if err != nil {
			return nil, fmt.Errorf(
				"decode mapping compatibility %d coordinate: %w",
				index,
				err,
			)
		}
		result = append(result, mapping)
	}
	return result, nil
}
