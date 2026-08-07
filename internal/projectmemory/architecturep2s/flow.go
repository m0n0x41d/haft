package architecturep2s

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type FlowPositionKind string

const (
	FlowPositionEntity FlowPositionKind = "entity"
	FlowPositionFlow   FlowPositionKind = "flow"
)

func (kind FlowPositionKind) valid() bool {
	return kind == FlowPositionEntity || kind == FlowPositionFlow
}

type FlowPositionRef struct {
	kind FlowPositionKind
	id   string
}

func NewFlowPositionRef(
	kind FlowPositionKind,
	id string,
) (FlowPositionRef, error) {
	if !kind.valid() || id == "" {
		return FlowPositionRef{}, fmt.Errorf(
			"transformation-flow position requires kind and identity",
		)
	}
	return FlowPositionRef{kind: kind, id: id}, nil
}

func (reference FlowPositionRef) Kind() FlowPositionKind {
	return reference.kind
}

func (reference FlowPositionRef) ID() string { return reference.id }

func (reference FlowPositionRef) key() string {
	return string(reference.kind) + "|" + reference.id
}

type FlowFiller struct {
	ordinal  uint32
	position FlowPositionRef
}

func NewFlowFiller(
	ordinal uint32,
	position FlowPositionRef,
) (FlowFiller, error) {
	if !position.kind.valid() || position.id == "" {
		return FlowFiller{}, fmt.Errorf(
			"transformation-flow filler requires an exact position",
		)
	}
	return FlowFiller{ordinal: ordinal, position: position}, nil
}

func (filler FlowFiller) Ordinal() uint32 { return filler.ordinal }

func (filler FlowFiller) Position() FlowPositionRef {
	return filler.position
}

type FlowSlot struct {
	name    string
	fillers []FlowFiller
}

func NewFlowSlot(name string, fillers []FlowFiller) (FlowSlot, error) {
	if name == "" || len(fillers) == 0 {
		return FlowSlot{}, fmt.Errorf(
			"transformation-flow slot requires name and fillers",
		)
	}
	canonical := append([]FlowFiller(nil), fillers...)
	sort.Slice(canonical, func(left int, right int) bool {
		return canonical[left].ordinal < canonical[right].ordinal
	})
	for index, filler := range canonical {
		if !filler.position.kind.valid() || filler.position.id == "" {
			return FlowSlot{}, fmt.Errorf(
				"transformation-flow slot %q has an invalid filler",
				name,
			)
		}
		if index > 0 &&
			canonical[index-1].ordinal == filler.ordinal {
			return FlowSlot{}, fmt.Errorf(
				"transformation-flow slot %q repeats ordinal %d",
				name,
				filler.ordinal,
			)
		}
	}
	return FlowSlot{name: name, fillers: canonical}, nil
}

func (slot FlowSlot) Name() string { return slot.name }

func (slot FlowSlot) Fillers() []FlowFiller {
	return append([]FlowFiller(nil), slot.fillers...)
}

type FlowRelation struct {
	id    string
	slots []FlowSlot
}

func NewFlowRelation(id string, slots []FlowSlot) (FlowRelation, error) {
	if id == "" || len(slots) < 2 {
		return FlowRelation{}, fmt.Errorf(
			"transformation-flow relation requires identity and at least two slots",
		)
	}
	canonical := append([]FlowSlot(nil), slots...)
	sort.Slice(canonical, func(left int, right int) bool {
		return canonical[left].Name() < canonical[right].Name()
	})
	for index, slot := range canonical {
		if slot.Name() == "" || len(slot.Fillers()) == 0 {
			return FlowRelation{}, fmt.Errorf(
				"transformation-flow relation %q has an invalid slot",
				id,
			)
		}
		if index > 0 && canonical[index-1].Name() == slot.Name() {
			return FlowRelation{}, fmt.Errorf(
				"transformation-flow relation %q repeats slot %q",
				id,
				slot.Name(),
			)
		}
	}
	return FlowRelation{id: id, slots: canonical}, nil
}

func (relation FlowRelation) ID() string { return relation.id }

func (relation FlowRelation) Slots() []FlowSlot {
	return append([]FlowSlot(nil), relation.slots...)
}

type TransformationFlowNetwork struct {
	relations      []FlowRelation
	canonicalBytes []byte
}

func NewTransformationFlowNetwork(
	relations []FlowRelation,
) (TransformationFlowNetwork, error) {
	if len(relations) == 0 {
		return TransformationFlowNetwork{}, fmt.Errorf(
			"transformation-flow network requires at least one relation",
		)
	}
	canonical := append([]FlowRelation(nil), relations...)
	sort.Slice(canonical, func(left int, right int) bool {
		return canonical[left].ID() < canonical[right].ID()
	})
	relationIDs := make(map[string]struct{}, len(canonical))
	for index, relation := range canonical {
		if relation.ID() == "" || len(relation.Slots()) < 2 {
			return TransformationFlowNetwork{}, fmt.Errorf(
				"transformation-flow relation %d is invalid",
				index,
			)
		}
		if _, found := relationIDs[relation.ID()]; found {
			return TransformationFlowNetwork{}, fmt.Errorf(
				"transformation-flow network repeats %q",
				relation.ID(),
			)
		}
		relationIDs[relation.ID()] = struct{}{}
	}
	if err := validateFlowReferences(canonical, relationIDs); err != nil {
		return TransformationFlowNetwork{}, err
	}
	encoded, err := encodeFlowNetwork(canonical)
	if err != nil {
		return TransformationFlowNetwork{}, err
	}
	return TransformationFlowNetwork{
		relations:      canonical,
		canonicalBytes: encoded,
	}, nil
}

func (network TransformationFlowNetwork) Relations() []FlowRelation {
	return append([]FlowRelation(nil), network.relations...)
}

func (network TransformationFlowNetwork) CanonicalBytes() []byte {
	return append([]byte(nil), network.canonicalBytes...)
}

type FlowTableRow struct {
	RelationID   string
	Slot         string
	Ordinal      uint32
	PositionKind FlowPositionKind
	PositionID   string
}

type FlowTableProjection struct {
	rows []FlowTableRow
}

func (network TransformationFlowNetwork) TableProjection() FlowTableProjection {
	rows := make([]FlowTableRow, 0)
	for _, relation := range network.relations {
		for _, slot := range relation.Slots() {
			for _, filler := range slot.Fillers() {
				rows = append(rows, FlowTableRow{
					RelationID:   relation.ID(),
					Slot:         slot.Name(),
					Ordinal:      filler.Ordinal(),
					PositionKind: filler.Position().Kind(),
					PositionID:   filler.Position().ID(),
				})
			}
		}
	}
	return FlowTableProjection{rows: rows}
}

func (projection FlowTableProjection) Rows() []FlowTableRow {
	return append([]FlowTableRow(nil), projection.rows...)
}

type FlowGraphNodeKind string

const (
	FlowGraphRelationNode FlowGraphNodeKind = "relation"
	FlowGraphPositionNode FlowGraphNodeKind = "position"
)

type FlowGraphNode struct {
	Kind         FlowGraphNodeKind
	ID           string
	PositionKind FlowPositionKind
}

type FlowGraphIncidence struct {
	RelationID   string
	Slot         string
	Ordinal      uint32
	PositionKind FlowPositionKind
	PositionID   string
}

type FlowGraphProjection struct {
	nodes      []FlowGraphNode
	incidences []FlowGraphIncidence
}

func (network TransformationFlowNetwork) GraphProjection() FlowGraphProjection {
	nodesByKey := make(map[string]FlowGraphNode)
	incidences := make([]FlowGraphIncidence, 0)
	for _, relation := range network.relations {
		relationNode := FlowGraphNode{
			Kind: FlowGraphRelationNode,
			ID:   relation.ID(),
		}
		nodesByKey["relation|"+relation.ID()] = relationNode
		for _, slot := range relation.Slots() {
			for _, filler := range slot.Fillers() {
				position := filler.Position()
				positionNode := FlowGraphNode{
					Kind:         FlowGraphPositionNode,
					ID:           position.ID(),
					PositionKind: position.Kind(),
				}
				nodesByKey["position|"+position.key()] = positionNode
				incidences = append(incidences, FlowGraphIncidence{
					RelationID:   relation.ID(),
					Slot:         slot.Name(),
					Ordinal:      filler.Ordinal(),
					PositionKind: position.Kind(),
					PositionID:   position.ID(),
				})
			}
		}
	}
	keys := make([]string, 0, len(nodesByKey))
	for key := range nodesByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	nodes := make([]FlowGraphNode, 0, len(keys))
	for _, key := range keys {
		nodes = append(nodes, nodesByKey[key])
	}
	return FlowGraphProjection{nodes: nodes, incidences: incidences}
}

func (projection FlowGraphProjection) Nodes() []FlowGraphNode {
	return append([]FlowGraphNode(nil), projection.nodes...)
}

func (projection FlowGraphProjection) Incidences() []FlowGraphIncidence {
	return append([]FlowGraphIncidence(nil), projection.incidences...)
}

func NetworkFromTable(
	projection FlowTableProjection,
) (TransformationFlowNetwork, error) {
	rows := projection.Rows()
	return networkFromRows(rows)
}

func NetworkFromGraph(
	projection FlowGraphProjection,
) (TransformationFlowNetwork, error) {
	if err := validateFlowGraphNodes(projection); err != nil {
		return TransformationFlowNetwork{}, err
	}
	incidences := projection.Incidences()
	rows := make([]FlowTableRow, 0, len(incidences))
	for _, incidence := range incidences {
		rows = append(rows, FlowTableRow(incidence))
	}
	return networkFromRows(rows)
}

func networkFromRows(rows []FlowTableRow) (TransformationFlowNetwork, error) {
	if len(rows) == 0 {
		return TransformationFlowNetwork{}, fmt.Errorf(
			"transformation-flow projection has no incidence rows",
		)
	}
	relationSlots := make(map[string]map[string][]FlowFiller)
	for _, row := range rows {
		position, err := NewFlowPositionRef(
			row.PositionKind,
			row.PositionID,
		)
		if err != nil {
			return TransformationFlowNetwork{}, err
		}
		filler, err := NewFlowFiller(row.Ordinal, position)
		if err != nil {
			return TransformationFlowNetwork{}, err
		}
		if row.RelationID == "" || row.Slot == "" {
			return TransformationFlowNetwork{}, fmt.Errorf(
				"transformation-flow projection row is incomplete",
			)
		}
		slots := relationSlots[row.RelationID]
		if slots == nil {
			slots = make(map[string][]FlowFiller)
			relationSlots[row.RelationID] = slots
		}
		slots[row.Slot] = append(slots[row.Slot], filler)
	}
	relationIDs := make([]string, 0, len(relationSlots))
	for relationID := range relationSlots {
		relationIDs = append(relationIDs, relationID)
	}
	sort.Strings(relationIDs)
	relations := make([]FlowRelation, 0, len(relationIDs))
	for _, relationID := range relationIDs {
		slotNames := make([]string, 0, len(relationSlots[relationID]))
		for slotName := range relationSlots[relationID] {
			slotNames = append(slotNames, slotName)
		}
		sort.Strings(slotNames)
		slots := make([]FlowSlot, 0, len(slotNames))
		for _, slotName := range slotNames {
			slot, err := NewFlowSlot(
				slotName,
				relationSlots[relationID][slotName],
			)
			if err != nil {
				return TransformationFlowNetwork{}, err
			}
			slots = append(slots, slot)
		}
		relation, err := NewFlowRelation(relationID, slots)
		if err != nil {
			return TransformationFlowNetwork{}, err
		}
		relations = append(relations, relation)
	}
	return NewTransformationFlowNetwork(relations)
}

func validateFlowReferences(
	relations []FlowRelation,
	relationIDs map[string]struct{},
) error {
	for _, relation := range relations {
		for _, slot := range relation.Slots() {
			for _, filler := range slot.Fillers() {
				position := filler.Position()
				if position.Kind() != FlowPositionFlow {
					continue
				}
				if _, found := relationIDs[position.ID()]; !found {
					return fmt.Errorf(
						"transformation-flow %q references missing nested flow %q",
						relation.ID(),
						position.ID(),
					)
				}
			}
		}
	}
	return nil
}

func validateFlowGraphNodes(projection FlowGraphProjection) error {
	nodes := projection.Nodes()
	incidences := projection.Incidences()
	nodeKeys := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		key, err := flowGraphNodeKey(node)
		if err != nil {
			return err
		}
		if _, found := nodeKeys[key]; found {
			return fmt.Errorf(
				"transformation-flow graph repeats node %q",
				key,
			)
		}
		nodeKeys[key] = struct{}{}
	}
	for _, incidence := range incidences {
		relationKey := "relation|" + incidence.RelationID
		positionKey := "position|" + string(incidence.PositionKind) +
			"|" + incidence.PositionID
		_, relationFound := nodeKeys[relationKey]
		_, positionFound := nodeKeys[positionKey]
		if !relationFound || !positionFound {
			return fmt.Errorf(
				"transformation-flow graph incidence has a missing node",
			)
		}
	}
	return nil
}

func flowGraphNodeKey(node FlowGraphNode) (string, error) {
	switch node.Kind {
	case FlowGraphRelationNode:
		if node.ID == "" || node.PositionKind != "" {
			return "", fmt.Errorf(
				"transformation-flow relation node is invalid",
			)
		}
		return "relation|" + node.ID, nil
	case FlowGraphPositionNode:
		if node.ID == "" || !node.PositionKind.valid() {
			return "", fmt.Errorf(
				"transformation-flow position node is invalid",
			)
		}
		return "position|" + string(node.PositionKind) + "|" + node.ID, nil
	default:
		return "", fmt.Errorf(
			"transformation-flow graph node kind %q is invalid",
			node.Kind,
		)
	}
}

func encodeFlowNetwork(relations []FlowRelation) ([]byte, error) {
	type canonicalFiller struct {
		Ordinal      uint32           `json:"ordinal"`
		PositionKind FlowPositionKind `json:"position_kind"`
		PositionID   string           `json:"position_id"`
	}
	type canonicalSlot struct {
		Name    string            `json:"name"`
		Fillers []canonicalFiller `json:"fillers"`
	}
	type canonicalRelation struct {
		ID    string          `json:"id"`
		Slots []canonicalSlot `json:"slots"`
	}
	payload := make([]canonicalRelation, 0, len(relations))
	for _, relation := range relations {
		slots := make([]canonicalSlot, 0, len(relation.Slots()))
		for _, slot := range relation.Slots() {
			fillers := make([]canonicalFiller, 0, len(slot.Fillers()))
			for _, filler := range slot.Fillers() {
				fillers = append(fillers, canonicalFiller{
					Ordinal:      filler.Ordinal(),
					PositionKind: filler.Position().Kind(),
					PositionID:   filler.Position().ID(),
				})
			}
			slots = append(slots, canonicalSlot{
				Name:    slot.Name(),
				Fillers: fillers,
			})
		}
		payload = append(payload, canonicalRelation{
			ID:    relation.ID(),
			Slots: slots,
		})
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(
			"encode transformation-flow network: %w",
			err,
		)
	}
	return encoded, nil
}

func SameTransformationFlowNetwork(
	left TransformationFlowNetwork,
	right TransformationFlowNetwork,
) bool {
	return bytes.Equal(left.CanonicalBytes(), right.CanonicalBytes())
}
