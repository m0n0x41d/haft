package cli

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/initplanning"
)

type publicDeprecatedSkillRemoval struct {
	kind           publicLegacyCleanupRemovalKind
	host           initplanning.HostID
	path           string
	anchorRoot     string
	relativePath   string
	expectedDigest string
	expectedMode   fs.FileMode
}

type publicDeprecatedSkillCleanupPlan struct {
	removals []publicDeprecatedSkillRemoval
	recovery []string
}

type publicLegacyCleanupRemovalKind string

const (
	publicDeprecatedSkillTreeRemoval publicLegacyCleanupRemovalKind = "skill_tree"
	publicLegacyCommandFileRemoval   publicLegacyCleanupRemovalKind = "command_file"
)

type publicDeprecatedSkillCleanupPreview struct {
	Paths []string
}

type publicLegacyCommandCleanupPreview struct {
	Paths []string
}

func compilePublicDeprecatedSkillCleanupPlan(
	request publicInitRequest,
	runtime currentHostPublicationRuntime,
	hermes publicHermesPlan,
	hasHermes bool,
) (publicDeprecatedSkillCleanupPlan, error) {
	roots, err := publicSelectedSkillRoots(
		request,
		runtime,
		hermes,
		hasHermes,
	)
	if err != nil {
		return publicDeprecatedSkillCleanupPlan{}, err
	}
	removals := make([]publicDeprecatedSkillRemoval, 0)
	for _, root := range roots {
		for _, name := range deprecatedSkillDirs {
			path := filepath.Join(root, name)
			removal, err := newPublicLegacyCleanupRemoval(
				publicDeprecatedSkillTreeRemoval,
				"",
				path,
				request.projectRoot,
				runtime.userHomeRoot,
			)
			if err != nil {
				return publicDeprecatedSkillCleanupPlan{}, err
			}
			digest, mode, present, err := observePublicLegacyRemoval(
				removal,
			)
			if err != nil {
				return publicDeprecatedSkillCleanupPlan{}, err
			}
			if !present {
				continue
			}
			removal.expectedDigest = digest
			removal.expectedMode = mode
			removals = append(removals, removal)
		}
	}
	commandCandidates := publicSelectedLegacyCommandCandidates(
		request,
		runtime,
	)
	for _, candidate := range commandCandidates {
		removal, err := newPublicLegacyCleanupRemoval(
			publicLegacyCommandFileRemoval,
			candidate.host,
			candidate.path,
			request.projectRoot,
			runtime.userHomeRoot,
		)
		if err != nil {
			return publicDeprecatedSkillCleanupPlan{}, err
		}
		digest, mode, present, err := observePublicLegacyRemoval(removal)
		if err != nil {
			return publicDeprecatedSkillCleanupPlan{}, err
		}
		if !present {
			continue
		}
		removal.expectedDigest = digest
		removal.expectedMode = mode
		removals = append(removals, removal)
	}
	sort.Slice(removals, func(left int, right int) bool {
		return removals[left].path < removals[right].path
	})
	return publicDeprecatedSkillCleanupPlan{
		removals: removals,
		recovery: publicInitRecoveryArgv(request),
	}, nil
}

func newPublicLegacyCleanupRemoval(
	kind publicLegacyCleanupRemovalKind,
	host initplanning.HostID,
	path string,
	projectRoot string,
	userHomeRoot string,
) (publicDeprecatedSkillRemoval, error) {
	path = filepath.Clean(path)
	anchors := []string{
		filepath.Clean(projectRoot),
		filepath.Clean(userHomeRoot),
	}
	for _, anchor := range anchors {
		relative, err := filepath.Rel(anchor, path)
		if err != nil || !publicCleanupRelativePath(relative) {
			continue
		}
		return publicDeprecatedSkillRemoval{
			kind:         kind,
			host:         host,
			path:         path,
			anchorRoot:   anchor,
			relativePath: relative,
		}, nil
	}
	anchor := filepath.Dir(path)
	info, err := os.Lstat(anchor)
	if err != nil {
		return publicDeprecatedSkillRemoval{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return publicDeprecatedSkillRemoval{}, fmt.Errorf(
			"legacy cleanup external anchor is not a physical directory: %s",
			anchor,
		)
	}
	relative := filepath.Base(path)
	return publicDeprecatedSkillRemoval{
		kind:         kind,
		host:         host,
		path:         path,
		anchorRoot:   anchor,
		relativePath: relative,
	}, nil
}

func publicCleanupRelativePath(path string) bool {
	path = filepath.Clean(path)
	if path == "." || filepath.IsAbs(path) || path == ".." {
		return false
	}
	return !strings.HasPrefix(
		path,
		".."+string(filepath.Separator),
	)
}

type publicLegacyCommandCandidate struct {
	host initplanning.HostID
	path string
}

func publicSelectedLegacyCommandCandidates(
	request publicInitRequest,
	runtime currentHostPublicationRuntime,
) []publicLegacyCommandCandidate {
	roots := make(map[string]initplanning.HostID)
	for _, binding := range request.hostBindings {
		if !slices.Contains(
			binding.components.Values(),
			initplanning.ComponentSkills,
		) {
			continue
		}
		switch binding.host {
		case initplanning.HostClaude:
			root := filepath.Join(
				runtime.userHomeRoot,
				".claude",
				"commands",
			)
			if binding.scope == initplanning.ScopeProject {
				root = filepath.Join(
					request.projectRoot,
					".claude",
					"commands",
				)
			}
			roots[root] = binding.host
		case initplanning.HostCodex:
			roots[filepath.Join(
				runtime.userHomeRoot,
				".codex",
				"prompts",
			)] = binding.host
		}
	}
	candidates := make(
		[]publicLegacyCommandCandidate,
		0,
		len(roots)*len(deprecatedCommands),
	)
	for root, host := range roots {
		for _, name := range deprecatedCommands {
			candidates = append(candidates, publicLegacyCommandCandidate{
				host: host,
				path: filepath.Join(root, name+".md"),
			})
		}
	}
	sort.Slice(candidates, func(left int, right int) bool {
		return candidates[left].path < candidates[right].path
	})
	return candidates
}

func publicSelectedSkillRoots(
	request publicInitRequest,
	runtime currentHostPublicationRuntime,
	hermes publicHermesPlan,
	hasHermes bool,
) ([]string, error) {
	roots := make(map[string]struct{})
	if request.agentSkills != publicAgentSkillsNone {
		_, root, err := publicAgentSkillsLocation(
			request,
			runtime.userHomeRoot,
		)
		if err != nil {
			return nil, err
		}
		roots[root] = struct{}{}
	}
	if hasHermes {
		roots[hermes.skillsRoot] = struct{}{}
	}
	bundle, err := currentSkillSourceBundle()
	if err != nil {
		return nil, err
	}
	candidates, err := currentStandardSkillCandidates(
		request.projectRoot,
		bundle,
		runtime,
	)
	if err != nil {
		return nil, err
	}
	for _, binding := range request.hostBindings {
		if !slices.Contains(
			binding.components.Values(),
			initplanning.ComponentSkills,
		) {
			continue
		}
		for _, candidate := range candidates {
			if candidate.host == binding.host &&
				candidate.scope == binding.scope {
				roots[candidate.targetRoot] = struct{}{}
			}
		}
	}
	values := make([]string, 0, len(roots))
	for root := range roots {
		values = append(values, root)
	}
	sort.Strings(values)
	return values, nil
}

func observePublicRemovalTree(
	removal publicDeprecatedSkillRemoval,
) (string, fs.FileMode, bool, error) {
	scopedRoot, err := openPublicCleanupAnchor(removal)
	if err != nil {
		return "", 0, false, err
	}
	rootInfo, err := scopedRoot.Lstat(removal.relativePath)
	if os.IsNotExist(err) {
		closeErr := scopedRoot.Close()
		return "", 0, false, closeErr
	}
	if err != nil {
		_ = scopedRoot.Close()
		return "", 0, false, err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		_ = scopedRoot.Close()
		return "", 0, false, fmt.Errorf(
			"deprecated skill cleanup path contains a symlink: %s",
			removal.path,
		)
	}
	if !rootInfo.IsDir() {
		_ = scopedRoot.Close()
		return "", 0, false, fmt.Errorf(
			"deprecated skill cleanup path is not a directory: %s",
			removal.path,
		)
	}
	treeDigest, digestErr := digestPublicRemovalTreeAt(
		scopedRoot,
		removal.relativePath,
	)
	closeErr := scopedRoot.Close()
	if digestErr != nil {
		return "", 0, false, digestErr
	}
	if closeErr != nil {
		return "", 0, false, closeErr
	}
	return treeDigest, rootInfo.Mode(), true, nil
}

func observePublicRemovalFile(
	removal publicDeprecatedSkillRemoval,
) (string, fs.FileMode, bool, error) {
	scopedRoot, err := openPublicCleanupAnchor(removal)
	if err != nil {
		return "", 0, false, err
	}
	info, err := scopedRoot.Lstat(removal.relativePath)
	if os.IsNotExist(err) {
		closeErr := scopedRoot.Close()
		return "", 0, false, closeErr
	}
	if err != nil {
		_ = scopedRoot.Close()
		return "", 0, false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		_ = scopedRoot.Close()
		return "", 0, false, fmt.Errorf(
			"legacy command cleanup path is a symlink: %s",
			removal.path,
		)
	}
	if !info.Mode().IsRegular() {
		_ = scopedRoot.Close()
		return "", 0, false, fmt.Errorf(
			"legacy command cleanup path is not a regular file: %s",
			removal.path,
		)
	}
	content, err := scopedRoot.ReadFile(removal.relativePath)
	if err != nil {
		_ = scopedRoot.Close()
		return "", 0, false, err
	}
	observed, err := scopedRoot.Lstat(removal.relativePath)
	if err != nil {
		_ = scopedRoot.Close()
		return "", 0, false, err
	}
	if observed.Mode() != info.Mode() ||
		observed.Size() != info.Size() {
		_ = scopedRoot.Close()
		return "", 0, false, fmt.Errorf(
			"legacy command cleanup path changed while it was observed: %s",
			removal.path,
		)
	}
	closeErr := scopedRoot.Close()
	if closeErr != nil {
		return "", 0, false, closeErr
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(content)), info.Mode(), true, nil
}

func openPublicCleanupAnchor(
	removal publicDeprecatedSkillRemoval,
) (*os.Root, error) {
	if !publicCleanupRelativePath(removal.relativePath) {
		return nil, fmt.Errorf(
			"legacy cleanup path is outside its anchor: %s",
			removal.path,
		)
	}
	anchorInfo, err := os.Lstat(removal.anchorRoot)
	if err != nil {
		return nil, err
	}
	if anchorInfo.Mode()&os.ModeSymlink != 0 || !anchorInfo.IsDir() {
		return nil, fmt.Errorf(
			"legacy cleanup anchor is not a physical directory: %s",
			removal.anchorRoot,
		)
	}
	scopedRoot, err := os.OpenRoot(removal.anchorRoot)
	if err != nil {
		return nil, err
	}
	if err := rejectPublicCleanupSymlinkAncestors(
		scopedRoot,
		removal.relativePath,
	); err != nil {
		_ = scopedRoot.Close()
		return nil, fmt.Errorf(
			"legacy cleanup path %s has an unsafe ancestor: %w",
			removal.path,
			err,
		)
	}
	return scopedRoot, nil
}

func rejectPublicCleanupSymlinkAncestors(
	scopedRoot *os.Root,
	path string,
) error {
	parent := filepath.Dir(filepath.Clean(path))
	if parent == "." {
		return nil
	}
	current := ""
	for _, component := range strings.Split(
		parent,
		string(filepath.Separator),
	) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := scopedRoot.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("ancestor %s is a symlink", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("ancestor %s is not a directory", current)
		}
	}
	return nil
}

func digestPublicRemovalTreeAt(
	scopedRoot *os.Root,
	treeRoot string,
) (string, error) {
	digest := sha256.New()
	err := fs.WalkDir(
		scopedRoot.FS(),
		filepath.ToSlash(treeRoot),
		func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			info, err := scopedRoot.Lstat(filepath.FromSlash(path))
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf(
					"deprecated skill cleanup path contains a symlink: %s",
					path,
				)
			}
			relative, err := filepath.Rel(
				treeRoot,
				filepath.FromSlash(path),
			)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(
				digest,
				"%s\x00%s\x00",
				filepath.ToSlash(relative),
				info.Mode().String(),
			); err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				content, err := scopedRoot.ReadFile(
					filepath.FromSlash(path),
				)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprintf(
					digest,
					"%x\x00",
					sha256.Sum256(content),
				); err != nil {
					return err
				}
			}
			return nil
		},
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", digest.Sum(nil)), nil
}

func previewPublicDeprecatedSkillCleanup(
	plan publicDeprecatedSkillCleanupPlan,
) publicDeprecatedSkillCleanupPreview {
	paths := make([]string, 0, len(plan.removals))
	for _, removal := range plan.removals {
		if removal.kind == publicDeprecatedSkillTreeRemoval {
			paths = append(paths, removal.path)
		}
	}
	return publicDeprecatedSkillCleanupPreview{Paths: paths}
}

func previewPublicLegacyCommandCleanup(
	plan publicDeprecatedSkillCleanupPlan,
) publicLegacyCommandCleanupPreview {
	paths := make([]string, 0, len(plan.removals))
	for _, removal := range plan.removals {
		if removal.kind == publicLegacyCommandFileRemoval {
			paths = append(paths, removal.path)
		}
	}
	return publicLegacyCommandCleanupPreview{Paths: paths}
}

func applyPublicDeprecatedSkillCleanupPlan(
	ctx context.Context,
	plan publicDeprecatedSkillCleanupPlan,
) (publicExactFileReceipt, error) {
	if ctx == nil {
		return publicExactFileReceipt{},
			fmt.Errorf("deprecated skill cleanup context is required")
	}
	paths := make([]string, len(plan.removals))
	for index, removal := range plan.removals {
		paths[index] = removal.path
	}
	if err := ctx.Err(); err != nil {
		failed := ""
		if len(paths) > 0 {
			failed = paths[0]
		}
		return publicExactFileReceipt{
			failed:    failed,
			untouched: slices.Clone(paths),
			retry:     slices.Clone(paths),
			recovery:  slices.Clone(plan.recovery),
		}, err
	}
	for _, removal := range plan.removals {
		if err := ctx.Err(); err != nil {
			return publicExactFileReceipt{
				failed:    removal.path,
				untouched: slices.Clone(paths),
				retry:     slices.Clone(paths),
				recovery:  slices.Clone(plan.recovery),
			}, err
		}
		digest, mode, present, err := observePublicLegacyRemoval(removal)
		if err != nil {
			return publicExactFileReceipt{
				failed:    removal.path,
				untouched: slices.Clone(paths),
				retry:     slices.Clone(paths),
				recovery:  slices.Clone(plan.recovery),
			}, err
		}
		if !present ||
			digest != removal.expectedDigest ||
			mode != removal.expectedMode {
			return publicExactFileReceipt{
					failed:    removal.path,
					untouched: slices.Clone(paths),
					retry:     slices.Clone(paths),
					recovery:  slices.Clone(plan.recovery),
				}, fmt.Errorf(
					"legacy cleanup path %s changed after preview; nothing was removed",
					removal.path,
				)
		}
	}
	completed := make([]string, 0, len(plan.removals))
	for index, removal := range plan.removals {
		if err := ctx.Err(); err != nil {
			return publicExactFileReceipt{
				completed: slices.Clone(completed),
				failed:    removal.path,
				untouched: slices.Clone(paths[index:]),
				retry:     slices.Clone(paths[index:]),
				recovery:  slices.Clone(plan.recovery),
			}, err
		}
		if err := applyPublicLegacyRemoval(removal); err != nil {
			return publicExactFileReceipt{
				completed: slices.Clone(completed),
				failed:    removal.path,
				untouched: slices.Clone(paths[index+1:]),
				retry:     slices.Clone(paths[index:]),
				recovery:  slices.Clone(plan.recovery),
			}, err
		}
		completed = append(completed, removal.path)
	}
	return publicExactFileReceipt{
		completed: completed,
		recovery:  slices.Clone(plan.recovery),
	}, nil
}

func observePublicLegacyRemoval(
	removal publicDeprecatedSkillRemoval,
) (string, fs.FileMode, bool, error) {
	switch removal.kind {
	case publicDeprecatedSkillTreeRemoval:
		return observePublicRemovalTree(removal)
	case publicLegacyCommandFileRemoval:
		return observePublicRemovalFile(removal)
	default:
		return "", 0, false, fmt.Errorf(
			"legacy cleanup removal kind %q is unsupported",
			removal.kind,
		)
	}
}

func applyPublicLegacyRemoval(
	removal publicDeprecatedSkillRemoval,
) error {
	scopedRoot, err := openPublicCleanupAnchor(removal)
	if err != nil {
		return err
	}
	info, err := scopedRoot.Lstat(removal.relativePath)
	if err != nil {
		_ = scopedRoot.Close()
		return err
	}
	switch removal.kind {
	case publicDeprecatedSkillTreeRemoval:
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			_ = scopedRoot.Close()
			return fmt.Errorf(
				"deprecated skill cleanup path is no longer a physical directory: %s",
				removal.path,
			)
		}
		err = scopedRoot.RemoveAll(removal.relativePath)
	case publicLegacyCommandFileRemoval:
		if info.Mode()&os.ModeSymlink != 0 ||
			!info.Mode().IsRegular() {
			_ = scopedRoot.Close()
			return fmt.Errorf(
				"legacy command cleanup path is no longer a regular file: %s",
				removal.path,
			)
		}
		err = scopedRoot.Remove(removal.relativePath)
	default:
		_ = scopedRoot.Close()
		return fmt.Errorf(
			"legacy cleanup removal kind %q is unsupported",
			removal.kind,
		)
	}
	return errors.Join(err, scopedRoot.Close())
}

func publicCleanupPlanHasKind(
	plan publicDeprecatedSkillCleanupPlan,
	kind publicLegacyCleanupRemovalKind,
) bool {
	for _, removal := range plan.removals {
		if removal.kind == kind {
			return true
		}
	}
	return false
}

func publicCleanupReceiptForKind(
	receipt publicExactFileReceipt,
	plan publicDeprecatedSkillCleanupPlan,
	kind publicLegacyCleanupRemovalKind,
) publicExactFileReceipt {
	paths := make(map[string]struct{})
	for _, removal := range plan.removals {
		if removal.kind == kind {
			paths[removal.path] = struct{}{}
		}
	}
	filter := func(values []string) []string {
		result := make([]string, 0, len(values))
		for _, value := range values {
			if _, selected := paths[value]; selected {
				result = append(result, value)
			}
		}
		return result
	}
	failed := ""
	if _, selected := paths[receipt.failed]; selected {
		failed = receipt.failed
	}
	return publicExactFileReceipt{
		completed: filter(receipt.completed),
		failed:    failed,
		untouched: filter(receipt.untouched),
		retry:     filter(receipt.retry),
		recovery:  slices.Clone(receipt.recovery),
	}
}

func publicInitRecoveryArgv(
	request publicInitRequest,
) []string {
	argv := []string{"haft", "init"}
	hosts := make(map[initplanning.HostID]struct{})
	for _, binding := range request.hostBindings {
		hosts[binding.host] = struct{}{}
	}
	hostOrder := []struct {
		host initplanning.HostID
		flag string
	}{
		{initplanning.HostClaude, "--claude"},
		{initplanning.HostCursor, "--cursor"},
		{initplanning.HostGemini, "--gemini"},
		{initplanning.HostCodex, "--codex"},
		{initplanning.HostAir, "--air"},
		{initplanning.HostOpenCode, "--opencode"},
		{initplanning.HostZed, "--zed"},
		{initplanning.HostAntigravity, "--agy"},
		{initplanning.HostPi, "--pi"},
		{initplanning.HostGrok, "--grok"},
	}
	for _, candidate := range hostOrder {
		if _, selected := hosts[candidate.host]; selected {
			argv = append(argv, candidate.flag)
		}
	}
	if request.hostMode == publicHostMCPOnly {
		argv = append(argv, "--mcp-only")
	}
	if request.hermes.kind == publicHermesConfigure {
		argv = append(argv, "--hermes")
		if request.hermes.homeInput != "" {
			argv = append(
				argv,
				"--hermes-home",
				request.hermes.homeInput,
			)
		}
		if request.hermes.profileInput != "" {
			argv = append(
				argv,
				"--profile",
				request.hermes.profileInput,
			)
		}
	}
	if request.agentSkills != publicAgentSkillsNone {
		argv = append(argv, "--agents")
	}
	if request.core == publicCoreOnly {
		argv = append(argv, "--core-only")
	}
	if publicInitUsesLocalScope(request) {
		argv = append(argv, "--local")
	}
	if request.instructions == publicInstructionsOmit {
		argv = append(argv, "--no-file-instructions")
	}
	if request.profileScope.kind == publicProfileScopeExact {
		argv = append(argv, "--scope-id", request.profileScope.scopeID)
	}
	return argv
}

func publicInitUsesLocalScope(
	request publicInitRequest,
) bool {
	return request.local
}
