package identityreconciliation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

type preparedRequest struct {
	request               Request
	operation             typedmemory.IdentityReconciliationOperation
	context               typedmemory.BoundedContextRef
	primary               typedmemory.EntityID
	related               []typedmemory.EntityID
	participantRole       string
	redirectKind          string
	changeSetBytes        []byte
	changeSetDigest       typedmemory.SHA256Digest
	basisBytes            []byte
	basisDigest           typedmemory.SHA256Digest
	admissionBytes        []byte
	admissionDigest       typedmemory.SHA256Digest
	reconciliationBytes   []byte
	reconciliationDigest  typedmemory.SHA256Digest
	reconciliationRef     string
	eventRef              string
	eventDigest           typedmemory.SHA256Digest
	commitRef             string
	projectionJobRef      string
	nextRevision          typedmemory.GraphRevision
	materializationBytes  []byte
	materializationDigest typedmemory.SHA256Digest
}

func prepareRequest(request Request) (preparedRequest, error) {
	admission := request.Admission()
	basis := admission.Basis()
	operation, contextRef, primary, related, participantRole, redirectKind, err :=
		identityCoordinates(admission.Change())
	if err != nil {
		return preparedRequest{}, err
	}
	effect, err := typedmemory.NewApplyIdentityChange(admission.Change())
	if err != nil {
		return preparedRequest{}, err
	}
	changeSet, err := typedmemory.NewMemoryChangeSet([]typedmemory.MemoryChange{effect})
	if err != nil {
		return preparedRequest{}, err
	}
	changeSetBytes, err := changeSet.CanonicalBytes()
	if err != nil {
		return preparedRequest{}, err
	}
	changeSetDigest, err := changeSet.Digest()
	if err != nil {
		return preparedRequest{}, err
	}
	basisBytes := basis.CanonicalBytes()
	basisDigest := basis.Digest()
	admissionBytes := admission.CanonicalBytes()
	admissionDigest := admission.Digest()
	if len(basisBytes) == 0 || basisDigest.String() == "" ||
		len(admissionBytes) == 0 || admissionDigest.String() == "" {
		return preparedRequest{}, fmt.Errorf("reviewed identity-reconciliation admission is invalid")
	}
	nextRevision := typedmemory.NewGraphRevision(basis.GraphRevision().Value() + 1)
	reconciliationBytes := canonicalFields(
		"haft.identity-reconciliation.request.v1",
		request.Project().String(),
		request.IdempotencyKey().String(),
		string(operation),
		contextRef.String(),
		primary.String(),
		joinEntities(related),
		strconv.FormatUint(basis.GraphRevision().Value(), 10),
		basis.TypeEnvRef().String(),
		changeSetDigest.String(),
		string(changeSetBytes),
		basisDigest.String(),
		string(basisBytes),
		admissionDigest.String(),
		string(admissionBytes),
		basis.PayloadDigest().String(),
		basis.Provenance().String(),
	)
	reconciliationDigest := digestBytes(reconciliationBytes)
	reconciliationRef := derivedRef("identity-reconciliation", reconciliationDigest.String())
	commitRef := derivedRef(
		"typed-memory-commit",
		request.Project().String(),
		request.IdempotencyKey().String(),
		changeSetDigest.String(),
		strconv.FormatUint(nextRevision.Value(), 10),
	)
	eventDigest := storageDigestFields(
		"typed-memory-graph-event.v1",
		request.Project().String(),
		commitRef,
		strconv.FormatUint(basis.GraphRevision().Value(), 10),
		strconv.FormatUint(nextRevision.Value(), 10),
		basis.TypeEnvRef().String(),
		changeSetDigest.String(),
		string(changeSetBytes),
		string(operation),
		"non_binding_semantic_assertion",
		basis.Provenance().String(),
	)
	eventRef := derivedRef("typed-memory-event", eventDigest.String())
	projectionJobRef := derivedRef("typed-memory-projection-job", commitRef)
	materializationFields := []string{
		eventRef,
		commitRef,
		eventDigest.String(),
		reconciliationRef,
		reconciliationDigest.String(),
		changeSetDigest.String(),
		basisDigest.String(),
		admissionDigest.String(),
		string(operation),
		contextRef.String(),
		primary.String(),
		strconv.Itoa(len(related)),
	}
	for index, entity := range related {
		materializationFields = append(
			materializationFields,
			strconv.Itoa(index),
			participantRole,
			entity.String(),
			redirectKind,
		)
	}
	materializationBytes := storageCanonicalFields(
		"typed-memory-materialization-closure.v1",
		materializationFields...,
	)
	materializationDigest := digestBytes(materializationBytes)
	return preparedRequest{
		request:               request,
		operation:             operation,
		context:               contextRef,
		primary:               primary,
		related:               append([]typedmemory.EntityID(nil), related...),
		participantRole:       participantRole,
		redirectKind:          redirectKind,
		changeSetBytes:        changeSetBytes,
		changeSetDigest:       changeSetDigest,
		basisBytes:            basisBytes,
		basisDigest:           basisDigest,
		admissionBytes:        admissionBytes,
		admissionDigest:       admissionDigest,
		reconciliationBytes:   reconciliationBytes,
		reconciliationDigest:  reconciliationDigest,
		reconciliationRef:     reconciliationRef,
		eventRef:              eventRef,
		eventDigest:           eventDigest,
		commitRef:             commitRef,
		projectionJobRef:      projectionJobRef,
		nextRevision:          nextRevision,
		materializationBytes:  materializationBytes,
		materializationDigest: materializationDigest,
	}, nil
}

func identityCoordinates(
	change typedmemory.IdentityChange,
) (
	typedmemory.IdentityReconciliationOperation,
	typedmemory.BoundedContextRef,
	typedmemory.EntityID,
	[]typedmemory.EntityID,
	string,
	string,
	error,
) {
	switch value := change.(type) {
	case typedmemory.MergeEntities:
		return typedmemory.ReconciliationMergeEntities,
			value.Context(),
			value.Survivor(),
			value.Merged(),
			"merged_entity",
			"merge_redirect",
			nil
	case typedmemory.SplitEntity:
		return typedmemory.ReconciliationSplitEntity,
			value.Context(),
			value.Source(),
			value.Targets(),
			"split_target",
			"split_candidate",
			nil
	default:
		return "", typedmemory.BoundedContextRef{}, typedmemory.EntityID{}, nil, "", "",
			fmt.Errorf("identity-reconciliation service accepts only reviewed merge or split")
	}
}

func joinEntities(entities []typedmemory.EntityID) string {
	values := make([]string, 0, len(entities))
	for _, entity := range entities {
		values = append(values, entity.String())
	}
	return strings.Join(values, "\x1f")
}

func canonicalFields(domain string, fields ...string) []byte {
	return framedFields("haft.identity-reconciliation.canonical.v1", domain, fields...)
}

func storageCanonicalFields(domain string, fields ...string) []byte {
	return framedFields("haft.typedmemorystore.canonical.v1", domain, fields...)
}

func storageDigestFields(domain string, fields ...string) typedmemory.SHA256Digest {
	return digestBytes(framedFields("haft.typedmemorystore.digest.v1", domain, fields...))
}

func framedFields(prefix string, domain string, fields ...string) []byte {
	buffer := make([]byte, 0)
	appendField := func(value string) {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		buffer = append(buffer, length[:]...)
		buffer = append(buffer, value...)
	}
	appendField(prefix)
	appendField(domain)
	for _, field := range fields {
		appendField(field)
	}
	return buffer
}

func digestBytes(value []byte) typedmemory.SHA256Digest {
	digest := sha256.Sum256(value)
	result, _ := typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(digest[:]))
	return result
}

func derivedRef(domain string, fields ...string) string {
	digest := storageDigestFields(domain, fields...)
	return domain + ":" + strings.TrimPrefix(digest.String(), "sha256:")
}
