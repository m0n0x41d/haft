package cli

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	methodpkg "github.com/m0n0x41d/haft/internal/method"
	"gopkg.in/yaml.v3"
)

type processMethodPackCarrierObservation struct {
	MethodRef  string `json:"method_ref"`
	CarrierRef string `json:"carrier_ref"`
	Status     string `json:"status"`
	Digest     string `json:"digest,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

func processCheckMethodPackCarriers(
	projectRoot string,
	observedAt string,
	validUntil string,
) ProcessCheckResult {
	observations := processMethodPackCarrierObservations(projectRoot)
	var checked int
	var missing int
	var stale int
	var samples []string
	for _, observation := range observations {
		checked++
		switch observation.Status {
		case "missing":
			missing++
			samples = append(samples, processMethodPackCarrierSample(observation))
		case "stale":
			stale++
			samples = append(samples, processMethodPackCarrierSample(observation))
		}
	}
	if missing > 0 || stale > 0 {
		return processCheckResult(
			"methodpack_carrier_currentness",
			"MethodPack.carrier_refs",
			"builtin_methodpack_carriers",
			processCheckStatusDegraded,
			"medium",
			observedAt,
			validUntil,
			fmt.Sprintf("MethodPack carrier refs need currentness review: checked=%d missing=%d stale=%d.", checked, missing, stale),
			processSampleStrings(samples, 10),
			"Run haft init or refresh MethodPack carriers, then rerun haft process check; carrier findings are advisory and do not create method authority.",
		)
	}
	return processCheckResult(
		"methodpack_carrier_currentness",
		"MethodPack.carrier_refs",
		"builtin_methodpack_carriers",
		processCheckStatusPass,
		"low",
		observedAt,
		validUntil,
		fmt.Sprintf("MethodPack carrier refs are present and current: checked=%d.", checked),
		processMethodPackCarrierDigestEvidence(observations),
		"No action.",
	)
}

func processMethodPackCarrierObservations(projectRoot string) []processMethodPackCarrierObservation {
	catalog := methodpkg.BuiltinCatalog()
	observations := make([]processMethodPackCarrierObservation, 0, len(catalog.Methods)*2)
	for _, definition := range catalog.Methods {
		for _, carrierRef := range definition.CarrierRefs {
			observations = append(observations, processMethodPackCarrierObservationFor(projectRoot, definition, carrierRef))
		}
	}
	return observations
}

func processMethodPackCarrierObservationFor(
	projectRoot string,
	definition methodpkg.Definition,
	carrierRef string,
) processMethodPackCarrierObservation {
	observation := processMethodPackCarrierObservation{
		MethodRef:  definition.ID,
		CarrierRef: carrierRef,
	}
	if processCarrierRefExternal(carrierRef) {
		observation.Status = "external_unchecked"
		observation.Reason = "external carrier refs are not fetched by process check"
		return observation
	}
	path := filepath.Join(projectRoot, filepath.Clean(carrierRef))
	data, err := os.ReadFile(path)
	if err != nil {
		observation.Status = "missing"
		observation.Reason = "carrier file is missing or unreadable"
		return observation
	}
	observation.Digest = processSHA256(data)
	if !strings.HasPrefix(filepath.ToSlash(carrierRef), ".haft/methods/") {
		observation.Status = "present"
		return observation
	}
	var carrier methodpkg.Definition
	if err := yaml.Unmarshal(data, &carrier); err != nil {
		observation.Status = "stale"
		observation.Reason = "method carrier YAML is unreadable"
		return observation
	}
	if carrier.ID != definition.ID ||
		carrier.Version != definition.Version ||
		carrier.SourcePosture.SourceEdition != definition.SourcePosture.SourceEdition ||
		carrier.Lifecycle.Status != definition.Lifecycle.Status {
		observation.Status = "stale"
		observation.Reason = "method carrier YAML does not match current builtin MethodPack identity, lifecycle, or source posture"
		return observation
	}
	observation.Status = "present"
	return observation
}

func processCarrierRefExternal(ref string) bool {
	lower := strings.ToLower(strings.TrimSpace(ref))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

func processSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + fmt.Sprintf("%x", sum[:])
}

func processMethodPackCarrierSample(observation processMethodPackCarrierObservation) string {
	return fmt.Sprintf("%s:%s status=%s reason=%s", observation.MethodRef, observation.CarrierRef, observation.Status, observation.Reason)
}

func processMethodPackCarrierDigestEvidence(observations []processMethodPackCarrierObservation) []string {
	evidence := make([]string, 0, len(observations))
	for _, observation := range observations {
		if observation.Digest == "" {
			continue
		}
		evidence = append(evidence, fmt.Sprintf("%s:%s %s", observation.MethodRef, observation.CarrierRef, observation.Digest))
	}
	return processSampleStrings(evidence, 8)
}
