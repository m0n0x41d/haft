package codebase

import "sort"

func (s *Scanner) scanCurrentCodeFiles(
	projectRoot string,
) (map[string]CodeFileState, error) {
	corpus, err := s.scanCurrentCodeCorpus(projectRoot)
	return corpus.states, err
}

func codeFilePaths(files map[string]CodeFileState) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
