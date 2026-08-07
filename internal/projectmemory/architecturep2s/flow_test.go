package architecturep2s

import (
	"reflect"
	"slices"
	"testing"
)

func TestRecursiveNaryFlowRoundTripsWithoutDirectionalPromotion(t *testing.T) {
	feed := p2sFlowPosition(t, FlowPositionEntity, "entity:feed")
	heat := p2sFlowPosition(t, FlowPositionEntity, "entity:heat")
	product := p2sFlowPosition(t, FlowPositionEntity, "entity:product")
	condition := p2sFlowPosition(t, FlowPositionEntity, "entity:condition")
	child := p2sFlowPosition(t, FlowPositionFlow, "flow:child")
	childRelation := p2sFlowRelation(t, "flow:child", []FlowSlot{
		p2sFlowSlot(t, "exposed-a", []FlowFiller{
			p2sFlowFiller(t, 0, feed),
		}),
		p2sFlowSlot(t, "exposed-b", []FlowFiller{
			p2sFlowFiller(t, 0, heat),
		}),
	})
	parentRelation := p2sFlowRelation(t, "flow:parent", []FlowSlot{
		p2sFlowSlot(t, "exposed-c", []FlowFiller{
			p2sFlowFiller(t, 1, child),
			p2sFlowFiller(t, 0, heat),
		}),
		p2sFlowSlot(t, "exposed-a", []FlowFiller{
			p2sFlowFiller(t, 0, product),
		}),
		p2sFlowSlot(t, "condition", []FlowFiller{
			p2sFlowFiller(t, 0, condition),
		}),
	})
	network, err := NewTransformationFlowNetwork(
		[]FlowRelation{parentRelation, childRelation},
	)
	if err != nil {
		t.Fatalf("NewTransformationFlowNetwork(): %v", err)
	}
	table := network.TableProjection()
	tableRows := table.Rows()
	slices.Reverse(tableRows)
	fromTable, err := NetworkFromTable(FlowTableProjection{rows: tableRows})
	if err != nil {
		t.Fatalf("NetworkFromTable(): %v", err)
	}
	graph := network.GraphProjection()
	graphNodes := graph.Nodes()
	graphIncidences := graph.Incidences()
	slices.Reverse(graphNodes)
	slices.Reverse(graphIncidences)
	fromGraph, err := NetworkFromGraph(FlowGraphProjection{
		nodes:      graphNodes,
		incidences: graphIncidences,
	})
	if err != nil {
		t.Fatalf("NetworkFromGraph(): %v", err)
	}
	if !SameTransformationFlowNetwork(network, fromTable) ||
		!SameTransformationFlowNetwork(network, fromGraph) {
		t.Fatal("graph/table projection changed recursive n-ary flow identity")
	}
	assertFlowProjectionHasNoPromotedSemantics(t)
}

func TestTransformationFlowRejectsDanglingNestedFlow(t *testing.T) {
	missing := p2sFlowPosition(t, FlowPositionFlow, "flow:missing")
	entity := p2sFlowPosition(t, FlowPositionEntity, "entity:one")
	relation := p2sFlowRelation(t, "flow:parent", []FlowSlot{
		p2sFlowSlot(t, "a", []FlowFiller{
			p2sFlowFiller(t, 0, missing),
		}),
		p2sFlowSlot(t, "b", []FlowFiller{
			p2sFlowFiller(t, 0, entity),
		}),
	})
	if _, err := NewTransformationFlowNetwork(
		[]FlowRelation{relation},
	); err == nil {
		t.Fatal("transformation-flow network accepted a dangling nested flow")
	}
}

func assertFlowProjectionHasNoPromotedSemantics(t *testing.T) {
	t.Helper()
	forbidden := []string{
		"Direction",
		"Cause",
		"Causality",
		"PartWhole",
		"Work",
		"WorkOrder",
	}
	types := []reflect.Type{
		reflect.TypeOf(FlowTableRow{}),
		reflect.TypeOf(FlowGraphNode{}),
		reflect.TypeOf(FlowGraphIncidence{}),
	}
	for _, valueType := range types {
		for _, field := range forbidden {
			if _, found := valueType.FieldByName(field); found {
				t.Fatalf(
					"%s exposes inferred semantic field %s",
					valueType.Name(),
					field,
				)
			}
		}
	}
}

func p2sFlowPosition(
	t *testing.T,
	kind FlowPositionKind,
	id string,
) FlowPositionRef {
	t.Helper()
	position, err := NewFlowPositionRef(kind, id)
	if err != nil {
		t.Fatalf("NewFlowPositionRef(): %v", err)
	}
	return position
}

func p2sFlowFiller(
	t *testing.T,
	ordinal uint32,
	position FlowPositionRef,
) FlowFiller {
	t.Helper()
	filler, err := NewFlowFiller(ordinal, position)
	if err != nil {
		t.Fatalf("NewFlowFiller(): %v", err)
	}
	return filler
}

func p2sFlowSlot(
	t *testing.T,
	name string,
	fillers []FlowFiller,
) FlowSlot {
	t.Helper()
	slot, err := NewFlowSlot(name, fillers)
	if err != nil {
		t.Fatalf("NewFlowSlot(): %v", err)
	}
	return slot
}

func p2sFlowRelation(
	t *testing.T,
	id string,
	slots []FlowSlot,
) FlowRelation {
	t.Helper()
	relation, err := NewFlowRelation(id, slots)
	if err != nil {
		t.Fatalf("NewFlowRelation(): %v", err)
	}
	return relation
}
