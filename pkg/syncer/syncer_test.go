package syncer_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentroom/agentroom/pkg/copier"
	"github.com/agentroom/agentroom/pkg/ignore"
	"github.com/agentroom/agentroom/pkg/index"
	"github.com/agentroom/agentroom/pkg/syncer"
	"github.com/agentroom/agentroom/pkg/workspace"
)

// setup creates a minimal main project + workspace copy and returns their paths.
func setup(t *testing.T) (mainPath, wsPath string) {
	t.Helper()
	root := t.TempDir()
	mainPath = filepath.Join(root, "main")
	wsPath = filepath.Join(root, "ws")
	if err := os.MkdirAll(mainPath, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"main.go":    "package main\nfunc main(){}\n",
		"README.md":  "hello\n",
		"src/util.go": "package src\n",
		".env":       "SECRET=abc\n",
	}
	for rel, content := range files {
		p := filepath.Join(mainPath, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(workspace.IgnorePath(mainPath), []byte(".env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &workspace.Config{
		Version:       workspace.ConfigVersion,
		ProjectName:   "main",
		MainPath:      mainPath,
		WorkspacePath: wsPath,
		CreatedAt:     time.Now().UTC(),
	}
	if err := workspace.SaveConfig(mainPath, cfg); err != nil {
		t.Fatal(err)
	}

	m, err := ignore.LoadFromFile(workspace.IgnorePath(mainPath))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := copier.CopyTree(mainPath, wsPath, m); err != nil {
		t.Fatal(err)
	}
	// .env should be excluded from the copy.
	if _, err := os.Stat(filepath.Join(wsPath, ".env")); !os.IsNotExist(err) {
		t.Fatalf(".env should not be copied, got err=%v", err)
	}
	idx, err := index.Build(mainPath, m)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Save(workspace.BaselinePath(mainPath)); err != nil {
		t.Fatal(err)
	}
	return mainPath, wsPath
}

func TestSyncAddModifyDelete(t *testing.T) {
	mainPath, wsPath := setup(t)

	// Modify, add, delete in workspace.
	if err := os.WriteFile(filepath.Join(wsPath, "main.go"), []byte("package main\nfunc main(){println(1)}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsPath, "NEW.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(wsPath, "README.md")); err != nil {
		t.Fatal(err)
	}

	plan, err := syncer.Analyze(syncer.AnalyzeOptions{MainPath: mainPath, WorkspacePath: wsPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.WorkspaceChanges) != 3 {
		t.Fatalf("expected 3 changes, got %d: %+v", len(plan.WorkspaceChanges), plan.WorkspaceChanges)
	}
	if len(plan.Conflicts) != 0 {
		t.Fatalf("unexpected conflicts: %v", plan.Conflicts)
	}

	if _, err := syncer.Apply(syncer.ApplyOptions{
		MainPath: mainPath, WorkspacePath: wsPath,
		Plan: plan, WithDelete: true,
	}); err != nil {
		t.Fatal(err)
	}

	// Verify main state.
	if data, _ := os.ReadFile(filepath.Join(mainPath, "main.go")); string(data) != "package main\nfunc main(){println(1)}\n" {
		t.Errorf("main.go not updated: %q", data)
	}
	if _, err := os.Stat(filepath.Join(mainPath, "NEW.txt")); err != nil {
		t.Errorf("NEW.txt not added: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mainPath, "README.md")); !os.IsNotExist(err) {
		t.Errorf("README.md should be deleted")
	}
	// .env should be untouched.
	if data, _ := os.ReadFile(filepath.Join(mainPath, ".env")); string(data) != "SECRET=abc\n" {
		t.Errorf(".env was disturbed: %q", data)
	}
}

func TestSyncConflictAborts(t *testing.T) {
	mainPath, wsPath := setup(t)
	// Modify same file in both.
	if err := os.WriteFile(filepath.Join(wsPath, "main.go"), []byte("ws-version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainPath, "main.go"), []byte("main-drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := syncer.Analyze(syncer.AnalyzeOptions{MainPath: mainPath, WorkspacePath: wsPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0] != "main.go" {
		t.Fatalf("expected conflict on main.go, got %v", plan.Conflicts)
	}
	if _, err := syncer.Apply(syncer.ApplyOptions{
		MainPath: mainPath, WorkspacePath: wsPath, Plan: plan, WithDelete: true,
	}); err == nil {
		t.Fatalf("expected conflict abort error")
	}
	// Force overwrites.
	if _, err := syncer.Apply(syncer.ApplyOptions{
		MainPath: mainPath, WorkspacePath: wsPath, Plan: plan, WithDelete: true, Force: true,
	}); err != nil {
		t.Fatalf("force apply failed: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(mainPath, "main.go")); string(data) != "ws-version\n" {
		t.Errorf("force should overwrite: %q", data)
	}
}

func TestUndoRestores(t *testing.T) {
	mainPath, wsPath := setup(t)

	origMain, _ := os.ReadFile(filepath.Join(mainPath, "main.go"))

	if err := os.WriteFile(filepath.Join(wsPath, "main.go"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsPath, "added.txt"), []byte("added\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(wsPath, "README.md")); err != nil {
		t.Fatal(err)
	}

	plan, err := syncer.Analyze(syncer.AnalyzeOptions{MainPath: mainPath, WorkspacePath: wsPath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := syncer.Apply(syncer.ApplyOptions{
		MainPath: mainPath, WorkspacePath: wsPath, Plan: plan, WithDelete: true,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := syncer.Undo(mainPath); err != nil {
		t.Fatal(err)
	}

	// main.go reverted.
	if data, _ := os.ReadFile(filepath.Join(mainPath, "main.go")); string(data) != string(origMain) {
		t.Errorf("main.go not restored: %q", data)
	}
	// added.txt removed.
	if _, err := os.Stat(filepath.Join(mainPath, "added.txt")); !os.IsNotExist(err) {
		t.Errorf("added.txt should be gone after undo")
	}
	// README.md restored.
	if _, err := os.Stat(filepath.Join(mainPath, "README.md")); err != nil {
		t.Errorf("README.md should be restored: %v", err)
	}
	// History empty.
	h, _ := syncer.LoadHistory(mainPath)
	if len(h.Entries) != 0 {
		t.Errorf("history should be empty after undo, got %d", len(h.Entries))
	}
}
