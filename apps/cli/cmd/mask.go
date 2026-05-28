package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agentroom/agentroom/pkg/ignore"
	"github.com/agentroom/agentroom/pkg/index"
	"github.com/agentroom/agentroom/pkg/logx"
	"github.com/agentroom/agentroom/pkg/mask"
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

type maskAddResult struct {
	Added      bool     `json:"added"`
	File       string   `json:"file"`
	Paths      []string `json:"paths"`
	TotalMasks int      `json:"total_masks"`
}

type maskListResult struct {
	Masks []mask.Mask `json:"masks"`
}

type maskRmResult struct {
	Removed   []string `json:"removed"`
	File      string   `json:"file"`
	Remaining int      `json:"remaining"`
}

func newMaskCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "mask",
		Short: "Mask specific fields inside YAML/JSON files",
		Long: `Path-level masking complements .agentignore: instead of hiding entire files,
mark specific dotted paths inside YAML/JSON files. The workspace copy shows a
placeholder at those paths, and sync always restores the original main value
(so an agent cannot leak the secret even by editing the placeholder).

Only .yaml/.yml/.json files are supported. Paths use dot syntax with array
indices: spring.datasource.password, users.0.email.`,
	}
	c.AddCommand(newMaskAddCmd(), newMaskListCmd(), newMaskRmCmd())
	return c
}

func newMaskAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <file> <path> [<path>...]",
		Short: "Add one or more masked paths for a YAML/JSON file",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMaskAdd(args[0], args[1:])
		},
	}
}

func newMaskListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured masks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMaskList()
		},
	}
}

func newMaskRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <file> [<path>]",
		Short: "Remove all masks for a file, or one specific path",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var path string
			if len(args) == 2 {
				path = args[1]
			}
			return runMaskRm(args[0], path)
		},
	}
}

func runMaskAdd(file string, paths []string) error {
	mainPath, err := workspace.FindMainRoot(".")
	if err != nil {
		return err
	}
	rel := filepath.ToSlash(file)
	if !mask.IsMaskable(rel) {
		return fmt.Errorf("unsupported file type %q: only .yaml/.yml/.json are maskable", rel)
	}
	abs := filepath.Join(mainPath, filepath.FromSlash(rel))
	content, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found in main project: %s", rel)
		}
		return err
	}
	kind := mask.ExtKind(rel)
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			return fmt.Errorf("empty path")
		}
		ok, err := mask.HasPath(content, kind, p)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", rel, err)
		}
		if !ok {
			return fmt.Errorf("path %q not found in %s", p, rel)
		}
	}

	cfg, err := mask.Load(mainPath)
	if err != nil {
		return err
	}
	merged := mergeMaskPaths(cfg, rel, paths)
	if err := mask.Save(mainPath, cfg); err != nil {
		return err
	}

	// Apply to workspace immediately if it exists.
	wcfg, _ := workspace.LoadConfig(mainPath)
	if wcfg != nil && wcfg.WorkspacePath != "" {
		if _, err := os.Stat(wcfg.WorkspacePath); err == nil {
			if err := refreshWorkspaceFileMask(mainPath, wcfg.WorkspacePath, rel, cfg); err != nil {
				return fmt.Errorf("refresh workspace: %w", err)
			}
			if err := rebuildBaseline(mainPath, cfg); err != nil {
				return fmt.Errorf("rebuild baseline: %w", err)
			}
		}
	}

	if jsonEnabled() {
		return outputJSON(maskAddResult{
			Added:      true,
			File:       rel,
			Paths:      merged,
			TotalMasks: totalPaths(cfg),
		})
	}
	logx.Infof("added %d mask path(s) for %s", len(paths), rel)
	return nil
}

func runMaskList() error {
	mainPath, err := workspace.FindMainRoot(".")
	if err != nil {
		return err
	}
	cfg, err := mask.Load(mainPath)
	if err != nil {
		return err
	}
	if cfg.Masks == nil {
		cfg.Masks = []mask.Mask{}
	}
	if jsonEnabled() {
		return outputJSON(maskListResult{Masks: cfg.Masks})
	}
	if len(cfg.Masks) == 0 {
		fmt.Println("(no masks configured)")
		return nil
	}
	files := make([]string, 0, len(cfg.Masks))
	byFile := map[string][]string{}
	for _, m := range cfg.Masks {
		files = append(files, m.File)
		byFile[m.File] = m.Paths
	}
	sort.Strings(files)
	for _, f := range files {
		fmt.Println(f)
		for _, p := range byFile[f] {
			fmt.Printf("  - %s\n", p)
		}
	}
	return nil
}

func runMaskRm(file, path string) error {
	mainPath, err := workspace.FindMainRoot(".")
	if err != nil {
		return err
	}
	rel := filepath.ToSlash(file)
	cfg, err := mask.Load(mainPath)
	if err != nil {
		return err
	}

	var removed []string
	idxFile := -1
	for i, m := range cfg.Masks {
		if m.File == rel {
			idxFile = i
			break
		}
	}
	if idxFile < 0 {
		return fmt.Errorf("no masks configured for %s", rel)
	}

	if path == "" {
		removed = append(removed, cfg.Masks[idxFile].Paths...)
		cfg.Masks = append(cfg.Masks[:idxFile], cfg.Masks[idxFile+1:]...)
	} else {
		paths := cfg.Masks[idxFile].Paths
		kept := paths[:0]
		for _, p := range paths {
			if p == path {
				removed = append(removed, p)
				continue
			}
			kept = append(kept, p)
		}
		if len(removed) == 0 {
			return fmt.Errorf("path %q not found in mask list for %s", path, rel)
		}
		if len(kept) == 0 {
			cfg.Masks = append(cfg.Masks[:idxFile], cfg.Masks[idxFile+1:]...)
		} else {
			cfg.Masks[idxFile].Paths = kept
		}
	}
	if err := mask.Save(mainPath, cfg); err != nil {
		return err
	}

	// Refresh workspace file: if mask still has entries for this file, re-apply
	// the new (smaller) set; otherwise restore from main (unmasked).
	wcfg, _ := workspace.LoadConfig(mainPath)
	if wcfg != nil && wcfg.WorkspacePath != "" {
		if _, err := os.Stat(wcfg.WorkspacePath); err == nil {
			if err := refreshWorkspaceFileMask(mainPath, wcfg.WorkspacePath, rel, cfg); err != nil {
				return fmt.Errorf("refresh workspace: %w", err)
			}
			if err := rebuildBaseline(mainPath, cfg); err != nil {
				return fmt.Errorf("rebuild baseline: %w", err)
			}
		}
	}

	remaining := 0
	for _, m := range cfg.Masks {
		if m.File == rel {
			remaining = len(m.Paths)
			break
		}
	}
	if jsonEnabled() {
		return outputJSON(maskRmResult{Removed: removed, File: rel, Remaining: remaining})
	}
	logx.Infof("removed %d mask path(s) from %s", len(removed), rel)
	return nil
}

// mergeMaskPaths inserts/updates a Mask entry for rel with paths (deduped).
// Returns the resulting path slice for rel.
func mergeMaskPaths(cfg *mask.Config, rel string, paths []string) []string {
	for i, m := range cfg.Masks {
		if m.File == rel {
			set := map[string]bool{}
			for _, p := range m.Paths {
				set[p] = true
			}
			for _, p := range paths {
				set[p] = true
			}
			out := make([]string, 0, len(set))
			for p := range set {
				out = append(out, p)
			}
			sort.Strings(out)
			cfg.Masks[i].Paths = out
			return out
		}
	}
	dedup := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if !dedup[p] {
			dedup[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	cfg.Masks = append(cfg.Masks, mask.Mask{File: rel, Paths: out})
	return out
}

func totalPaths(cfg *mask.Config) int {
	n := 0
	for _, m := range cfg.Masks {
		n += len(m.Paths)
	}
	return n
}

// refreshWorkspaceFileMask re-materializes a single workspace file from main:
// copies the raw content from main, then applies current masks for that file.
// (If no masks remain, the workspace file ends up unmasked, as expected.)
func refreshWorkspaceFileMask(mainPath, workspacePath, rel string, cfg *mask.Config) error {
	mainFile := filepath.Join(mainPath, filepath.FromSlash(rel))
	wsFile := filepath.Join(workspacePath, filepath.FromSlash(rel))
	raw, err := os.ReadFile(mainFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	paths := cfg.PathsFor(rel)
	out := raw
	if len(paths) > 0 {
		kind := mask.ExtKind(rel)
		if kind == "" {
			return nil
		}
		out, err = mask.Apply(raw, kind, paths)
		if err != nil {
			return err
		}
	}
	info, _ := os.Stat(mainFile)
	mode := os.FileMode(0o644)
	if info != nil {
		mode = info.Mode().Perm()
	}
	return workspace.AtomicWrite(wsFile, out, mode)
}

func rebuildBaseline(mainPath string, cfg *mask.Config) error {
	matcher, err := ignore.LoadFromFile(workspace.IgnorePath(mainPath))
	if err != nil {
		return err
	}
	idx, err := index.BuildWithMasks(mainPath, matcher, cfg)
	if err != nil {
		return err
	}
	return idx.Save(workspace.BaselinePath(mainPath))
}
