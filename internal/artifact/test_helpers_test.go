package artifact

func testVariant(title, weakestLink, novelty string) Variant {
	return Variant{
		Title:         title,
		WeakestLink:   weakestLink,
		NoveltyMarker: novelty,
	}
}
