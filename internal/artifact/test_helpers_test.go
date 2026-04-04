package artifact

func testVariant(title, weakestLink, novelty string) Variant {
	return Variant{
		Title:         title,
		WeakestLink:   weakestLink,
		NoveltyMarker: novelty,
	}
}

func testSteppingStoneVariant(title, weakestLink, novelty, basis string) Variant {
	variant := testVariant(title, weakestLink, novelty)
	variant.SteppingStone = true
	variant.SteppingStoneBasis = basis
	return variant
}
