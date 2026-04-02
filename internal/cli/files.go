package cli

import (
	"os"
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/protocol"
)

// scanProjectFiles walks the project directory and returns file info.
func scanProjectFiles(root string, maxItems int) []protocol.FileInfo {
	var files []protocol.FileInfo

	skipDirs := map[string]bool{
		".git": true, ".haft": true, "node_modules": true,
		"vendor": true, "__pycache__": true, ".venv": true,
		"target": true, "dist": true, "build": true,
		".context": true, ".quint": true,
	}

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if len(files) >= maxItems {
			return filepath.SkipAll
		}
		if info.IsDir() {
			name := info.Name()
			if skipDirs[name] || (len(name) > 0 && name[0] == '.') {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		files = append(files, protocol.FileInfo{
			Path: rel,
			Size: info.Size(),
		})
		return nil
	})

	return files
}
