package fpf

import "fmt"

type goldenRetrievalMetrics struct {
	Name       string
	Total      int
	Successful int
	TotalHits  int
	Failures   []string
}

func (metrics goldenRetrievalMetrics) failureSummary() string {
	if len(metrics.Failures) == 0 {
		return "none"
	}
	return fmt.Sprintf("%v", metrics.Failures)
}
