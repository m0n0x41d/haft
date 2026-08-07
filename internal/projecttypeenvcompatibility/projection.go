package projecttypeenvcompatibility

import (
	"fmt"
	"strconv"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func projectTypeEnv(environment typedmemory.TypeEnv) ([]semanticEntry, error) {
	if _, err := typedmemory.ParseTypeEnvRef(environment.Ref().String()); err != nil {
		return nil, fmt.Errorf("executable TypeEnv reference is required")
	}
	entries := make([]semanticEntry, 0)
	appendEntry := func(family Family, key string, material []byte) error {
		entry, err := newSemanticEntry(family, key, material)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
		return nil
	}
	if err := appendBoundedContexts(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendKindDefinitions(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendEntitySetDefinitions(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendKindSignatureDefinitions(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendKindClassificationSignatureDefinitions(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendRefKindDefinitions(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendContextKindAvailabilities(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendSubkindRelations(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendContextBridges(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendTypedRelationDeclarationFragments(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendValueShapes(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendValueBindings(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := appendConstraints(appendEntry, environment); err != nil {
		return nil, err
	}
	if err := sortSemanticEntries(entries); err != nil {
		return nil, err
	}
	return entries, nil
}

type semanticEntryAppender func(Family, string, []byte) error

func appendBoundedContexts(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, context := range environment.BoundedContexts() {
		writer := newCanonicalWriter("executable-typeenv.bounded-context.v1")
		writer.addString(context.Ref().String())
		if err := appendEntry(
			BoundedContextFamily,
			context.Ref().String(),
			writer.bytes(),
		); err != nil {
			return err
		}
	}
	return nil
}

func appendKindDefinitions(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, definition := range environment.KindDefinitions() {
		writer := newCanonicalWriter("executable-typeenv.kind-definition.v1")
		writer.addString(definition.ID().String())
		if err := appendEntry(
			KindDefinitionFamily,
			definition.ID().String(),
			writer.bytes(),
		); err != nil {
			return err
		}
	}
	return nil
}

func appendEntitySetDefinitions(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, definition := range environment.EntitySetDefinitions() {
		if definition.Ref().TypeEnv() != environment.Ref() {
			return fmt.Errorf(
				"EntitySet %q has unexpected owning TypeEnv %q",
				definition.Ref().Context().String(),
				definition.Ref().TypeEnv().String(),
			)
		}
		material, err := canonicalEntitySetDefinition(definition)
		if err != nil {
			return err
		}
		if err := appendEntry(
			EntitySetDefinitionFamily,
			definition.Ref().Context().String(),
			material,
		); err != nil {
			return err
		}
	}
	return nil
}

func appendKindSignatureDefinitions(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, definition := range environment.KindSignatureDefinitions() {
		if definition.ValueKind().TypeEnv() != environment.Ref() ||
			definition.EntitySet().TypeEnv() != environment.Ref() {
			return fmt.Errorf(
				"KindSignature %q has an unexpected owning TypeEnv",
				definition.Ref().String(),
			)
		}
		key := definition.Ref().Context().String()
		key += "/kind/"
		key += definition.ValueKind().ID().String()
		writer := newCanonicalWriter("executable-typeenv.kind-signature.v1")
		writer.addString(definition.ValueKind().ID().String())
		writer.addString(definition.Ref().Context().String())
		writer.addString(definition.Formality().String())
		assumptions := definition.Assumptions()
		writer.addUint64(uint64(len(assumptions)))
		for _, assumption := range assumptions {
			writer.addBytes(assumption.CanonicalBytes())
		}
		writer.addString(definition.DefinednessRule().String())
		writer.addString(definition.Evaluator().String())
		writer.addString(definition.EntitySet().Context().String())
		if err := appendEntry(
			KindSignatureDefinitionFamily,
			key,
			writer.bytes(),
		); err != nil {
			return err
		}
	}
	return nil
}

func appendKindClassificationSignatureDefinitions(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, definition := range environment.KindClassificationSignatureDefinitions() {
		if definition.LocalKind().TypeEnv() != environment.Ref() ||
			definition.CandidateValueKind().TypeEnv() != environment.Ref() {
			return fmt.Errorf(
				"current KindSignature %q has an unexpected owning TypeEnv",
				definition.Ref().String(),
			)
		}
		key := definition.LocalKind().Context().String()
		key += "/kind/"
		key += definition.LocalKind().ValueKind().ID().String()
		writer := newCanonicalWriter(
			"executable-typeenv.kind-classification-signature.v1",
		)
		writer.addString(definition.LocalKind().ValueKind().ID().String())
		writer.addString(definition.LocalKind().Context().String())
		writer.addString(definition.CandidateValueKind().ID().String())
		writer.addString(definition.Criterion().String())
		writer.addString(definition.SliceConditions().String())
		writer.addBytes(definition.ReferenceScheme().CanonicalBytes())
		dependencies := definition.Dependencies()
		writer.addUint64(uint64(len(dependencies)))
		for _, dependency := range dependencies {
			writer.addBytes(dependency.CanonicalBytes())
		}
		writer.addString(definition.Formality().String())
		writer.addBytes(definition.ExtentRule().CanonicalBytes())
		if err := appendEntry(
			KindClassificationSignatureFamily,
			key,
			writer.bytes(),
		); err != nil {
			return err
		}
	}
	return nil
}

func appendRefKindDefinitions(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, definition := range environment.RefKindDefinitions() {
		if definition.Ref().TypeEnv() != environment.Ref() ||
			definition.ValueKind().TypeEnv() != environment.Ref() {
			return fmt.Errorf(
				"RefKind %q has an unexpected owning TypeEnv",
				definition.Ref().ID().String(),
			)
		}
		writer := newCanonicalWriter("executable-typeenv.ref-kind-definition.v1")
		writer.addString(definition.Ref().ID().String())
		writer.addString(definition.ValueKind().ID().String())
		if err := appendEntry(
			RefKindDefinitionFamily,
			definition.Ref().ID().String(),
			writer.bytes(),
		); err != nil {
			return err
		}
	}
	return nil
}

func appendContextKindAvailabilities(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, availability := range environment.ContextKindAvailabilities() {
		key := availability.Context().String()
		key += "/kind/"
		key += availability.KindID().String()
		writer := newCanonicalWriter("executable-typeenv.context-kind-availability.v1")
		writer.addString(availability.Context().String())
		writer.addString(availability.KindID().String())
		if err := appendEntry(
			ContextKindAvailabilityFamily,
			key,
			writer.bytes(),
		); err != nil {
			return err
		}
	}
	return nil
}

func appendSubkindRelations(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, relation := range environment.SubkindRelations() {
		key := relation.Subkind().String()
		key += "/subkind-of/"
		key += relation.Superkind().String()
		writer := newCanonicalWriter("executable-typeenv.subkind-relation.v1")
		writer.addString(relation.Subkind().String())
		writer.addString(relation.Superkind().String())
		if err := appendEntry(SubkindRelationFamily, key, writer.bytes()); err != nil {
			return err
		}
	}
	return nil
}

func appendContextBridges(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, bridge := range environment.ContextBridges() {
		material := canonicalContextBridge(bridge)
		if err := appendEntry(
			ContextBridgeFamily,
			bridge.ID().String(),
			material,
		); err != nil {
			return err
		}
	}
	return nil
}

func appendTypedRelationDeclarationFragments(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, fragment := range environment.TypedRelationDeclarationFragments() {
		if fragment.Ref().TypeEnv() != environment.Ref() {
			return fmt.Errorf(
				"typed relation declaration fragment %q has unexpected owning TypeEnv %q",
				fragment.Ref().ID().String(),
				fragment.Ref().TypeEnv().String(),
			)
		}
		fragmentMaterial := canonicalTypedRelationDeclarationFragment(fragment)
		if err := appendEntry(
			TypedRelationDeclarationFragmentFamily,
			fragment.Ref().ID().String(),
			fragmentMaterial,
		); err != nil {
			return err
		}
		for _, slot := range fragment.Slots() {
			slotMaterial, err := canonicalRelationSlot(
				environment.Ref(),
				fragment.Ref().ID(),
				slot,
			)
			if err != nil {
				return err
			}
			key := fragment.Ref().ID().String()
			key += "/slot/"
			key += slot.SlotKind().String()
			if err := appendEntry(RelationSlotFamily, key, slotMaterial); err != nil {
				return err
			}
		}
	}
	return nil
}

func appendValueShapes(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, declaration := range environment.ValueShapes() {
		writer := newCanonicalWriter("executable-typeenv.value-shape.v1")
		writer.addString(declaration.Ref().String())
		writer.addString(string(declaration.Shape().Kind()))
		if err := appendEntry(
			ValueShapeFamily,
			declaration.Ref().String(),
			writer.bytes(),
		); err != nil {
			return err
		}
	}
	return nil
}

func appendValueBindings(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, binding := range environment.ValueBindings() {
		if binding.ValueKind().TypeEnv() != environment.Ref() {
			return fmt.Errorf(
				"value binding %q has unexpected owning TypeEnv %q",
				binding.ValueKind().ID().String(),
				binding.ValueKind().TypeEnv().String(),
			)
		}
		writer := newCanonicalWriter("executable-typeenv.value-binding.v1")
		writer.addString(binding.ValueKind().ID().String())
		writer.addString(binding.ValueShape().String())
		writer.addString(binding.Codec().String())
		if err := appendEntry(
			ValueBindingFamily,
			binding.ValueKind().ID().String(),
			writer.bytes(),
		); err != nil {
			return err
		}
	}
	return nil
}

func appendConstraints(
	appendEntry semanticEntryAppender,
	environment typedmemory.TypeEnv,
) error {
	for _, constraint := range environment.Constraints() {
		material, err := canonicalConstraint(environment.Ref(), constraint)
		if err != nil {
			return err
		}
		if err := appendEntry(
			ConstraintFamily,
			constraint.ID().String(),
			material,
		); err != nil {
			return err
		}
	}
	return nil
}

func canonicalEntitySetDefinition(
	definition typedmemory.EntitySetDefinition,
) ([]byte, error) {
	writer := newCanonicalWriter("executable-typeenv.entity-set-definition.v1")
	writer.addString(definition.Ref().Context().String())
	writer.addString(definition.EnumerationRule().String())
	switch policy := definition.CandidatePolicy().(type) {
	case typedmemory.PersistedEntitiesOnly:
		writer.addString("persisted_entities_only")
	case typedmemory.PriorBatchDeclarationsVisible:
		writer.addString("prior_batch_declarations_visible")
		writer.addString(policy.EvaluationRule().String())
	default:
		return nil, fmt.Errorf(
			"EntitySet %q has unsupported candidate policy %T",
			definition.Ref().Context().String(),
			definition.CandidatePolicy(),
		)
	}
	return writer.bytes(), nil
}

func canonicalContextBridge(bridge typedmemory.ContextBridge) []byte {
	writer := newCanonicalWriter("executable-typeenv.context-bridge.v1")
	writer.addString(bridge.ID().String())
	writer.addString(bridge.Source().Context().String())
	writer.addString(bridge.Source().Edition().String())
	writer.addString(bridge.Target().Context().String())
	writer.addString(bridge.Target().Edition().String())
	writer.addString(bridge.Mapping().SourceKind().String())
	writer.addString(bridge.Mapping().TargetKind().String())
	writer.addString(bridge.Direction().String())
	writer.addString(bridge.OrderCoverage().String())
	writer.addUint64(uint64(bridge.KindCongruence().Value()))
	lossNotes := bridge.LossNotes().Values()
	writer.addUint64(uint64(len(lossNotes)))
	for _, note := range lossNotes {
		writer.addString(note)
	}
	definedness := bridge.DefinednessArea().Values()
	writer.addUint64(uint64(len(definedness)))
	for _, condition := range definedness {
		writer.addString(condition)
	}
	return writer.bytes()
}

func canonicalTypedRelationDeclarationFragment(
	fragment typedmemory.TypedRelationDeclarationFragment,
) []byte {
	writer := newCanonicalWriter(
		"executable-typeenv.typed-relation-declaration-fragment.v1",
	)
	writer.addString(fragment.Ref().ID().String())
	writer.addString(fragment.Posture().String())
	contexts := fragment.Contexts()
	writer.addUint64(uint64(len(contexts)))
	for _, context := range contexts {
		writer.addString(context.String())
	}
	return writer.bytes()
}

func canonicalRelationSlot(
	environment typedmemory.TypeEnvRef,
	signature typedmemory.SignatureID,
	slot typedmemory.SlotSpec,
) ([]byte, error) {
	writer := newCanonicalWriter("executable-typeenv.relation-slot.v1")
	writer.addString(signature.String())
	writer.addString(slot.SlotKind().String())
	switch target := slot.Target().(type) {
	case typedmemory.ValueSlotTarget:
		if target.ValueKind().TypeEnv() != environment {
			return nil, fmt.Errorf(
				"relation slot %q has unexpected ValueKind owner %q",
				slot.SlotKind().String(),
				target.ValueKind().TypeEnv().String(),
			)
		}
		writer.addString("value")
		writer.addString(target.ValueKind().ID().String())
	case typedmemory.ReferenceSlotTarget:
		if target.ValueKind().TypeEnv() != environment ||
			target.ReferenceKind().TypeEnv() != environment {
			return nil, fmt.Errorf(
				"relation slot %q has an unexpected reference target owner",
				slot.SlotKind().String(),
			)
		}
		writer.addString("reference")
		writer.addString(target.ValueKind().ID().String())
		writer.addString(target.ReferenceKind().ID().String())
	default:
		return nil, fmt.Errorf(
			"relation slot %q has unsupported target %T",
			slot.SlotKind().String(),
			slot.Target(),
		)
	}
	cardinality := slot.Cardinality()
	maximum, bounded := cardinality.Maximum().BoundedValue()
	writer.addUint64(cardinality.Minimum())
	writer.addString(strconv.FormatBool(bounded))
	writer.addUint64(maximum)
	return writer.bytes(), nil
}

func canonicalConstraint(
	environment typedmemory.TypeEnvRef,
	constraint typedmemory.ConstraintRule,
) ([]byte, error) {
	writer := newCanonicalWriter("executable-typeenv.constraint.v1")
	writer.addString(constraint.ID().String())
	switch value := constraint.(type) {
	case typedmemory.KindDisjointConstraint:
		writer.addString("kind_disjoint")
		kinds := value.Kinds()
		writer.addUint64(uint64(len(kinds)))
		for _, kind := range kinds {
			writer.addString(kind.String())
		}
	case typedmemory.SlotGroupConstraint:
		if value.Signature().TypeEnv() != environment {
			return nil, unexpectedConstraintOwner(value.ID(), value.Signature().TypeEnv())
		}
		writer.addString("slot_group")
		writer.addString(value.Signature().ID().String())
		writer.addString(value.Mode().String())
		slots := value.Slots()
		writer.addUint64(uint64(len(slots)))
		for _, slot := range slots {
			writer.addString(slot.String())
		}
	case typedmemory.SlotCardinalityConstraint:
		if value.Signature().TypeEnv() != environment {
			return nil, unexpectedConstraintOwner(value.ID(), value.Signature().TypeEnv())
		}
		writer.addString("slot_cardinality")
		writer.addString(value.Signature().ID().String())
		writer.addString(value.Slot().String())
		cardinality := value.Cardinality()
		maximum, bounded := cardinality.Maximum().BoundedValue()
		writer.addUint64(cardinality.Minimum())
		writer.addString(strconv.FormatBool(bounded))
		writer.addUint64(maximum)
	case typedmemory.ReferenceSlotSubsetConstraint:
		if value.Signature().TypeEnv() != environment {
			return nil, unexpectedConstraintOwner(value.ID(), value.Signature().TypeEnv())
		}
		writer.addString("reference_slot_subset")
		writer.addString(value.Signature().ID().String())
		writer.addString(value.Subset().String())
		writer.addString(value.Superset().String())
	case typedmemory.ReferenceSlotPartitionConstraint:
		if value.Signature().TypeEnv() != environment {
			return nil, unexpectedConstraintOwner(value.ID(), value.Signature().TypeEnv())
		}
		writer.addString("reference_slot_partition")
		writer.addString(value.Signature().ID().String())
		writer.addString(value.Whole().String())
		parts := value.Parts()
		writer.addUint64(uint64(len(parts)))
		for _, part := range parts {
			writer.addString(part.String())
		}
	default:
		return nil, fmt.Errorf(
			"constraint %q has unsupported executable variant %T",
			constraint.ID().String(),
			constraint,
		)
	}
	return writer.bytes(), nil
}

func unexpectedConstraintOwner(
	id typedmemory.ConstraintID,
	owner typedmemory.TypeEnvRef,
) error {
	return fmt.Errorf(
		"constraint %q has unexpected relation owner %q",
		id.String(),
		owner.String(),
	)
}
