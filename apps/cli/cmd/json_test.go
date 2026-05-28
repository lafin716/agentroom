package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// captureJSON runs fn with jsonOutput=true, captures stdout, parses as JSON.
func captureJSON(t *testing.T, fn func() error) map[string]any {
	t.Helper()
	jsonOutput = true
	defer func() { jsonOutput = false }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w

	errCh := make(chan error, 1)
	go func() { errCh <- fn() }()

	doneCh := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(r)
		doneCh <- data
	}()

	runErr := <-errCh
	w.Close()
	os.Stdout = orig
	data := <-doneCh

	if runErr != nil {
		t.Fatalf("run error: %v\noutput: %s", runErr, data)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, data)
	}
	return out
}

// setupProject creates a temp project, chdir into it, returns cleanup.
func setupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	main := filepath.Join(dir, "proj")
	if err := os.MkdirAll(main, 0o755); err != nil {
		t.Fatal(err)
	}
	// Seed a couple of files.
	for rel, content := range map[string]string{
		"main.go":   "package main\nfunc main(){}\n",
		"README.md": "hi\n",
		".env":      "SECRET=1\n",
	} {
		if err := os.WriteFile(filepath.Join(main, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	old, _ := os.Getwd()
	if err := os.Chdir(main); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return main
}

func TestJSONOutputFullCycle(t *testing.T) {
	main := setupProject(t)

	// init
	initOut := captureJSON(t, func() error { return runInit(false) })
	if initOut["initialized"] != true {
		t.Errorf("init: expected initialized=true, got %+v", initOut)
	}
	for _, k := range []string{"agent_dir", "ignore_path", "config_path"} {
		if _, ok := initOut[k]; !ok {
			t.Errorf("init: missing key %q", k)
		}
	}

	// Append .env to .agentignore so it is excluded.
	ignorePath := filepath.Join(main, ".agentignore")
	cur, _ := os.ReadFile(ignorePath)
	_ = os.WriteFile(ignorePath, append(cur, []byte("\n.env\n")...), 0o644)

	// info (no workspace yet)
	infoOut := captureJSON(t, func() error { return runInfo() })
	if infoOut["initialized"] != true {
		t.Errorf("info: expected initialized=true, got %+v", infoOut)
	}
	if infoOut["cli_version"] == nil {
		t.Errorf("info: missing cli_version")
	}

	// copy
	dest := filepath.Join(t.TempDir(), "ws")
	copyOut := captureJSON(t, func() error { return runCopy(dest, false) })
	if copyOut["workspace_path"] == nil {
		t.Errorf("copy: missing workspace_path: %+v", copyOut)
	}
	if _, err := os.Stat(filepath.Join(dest, ".env")); !os.IsNotExist(err) {
		t.Errorf(".env should not have been copied")
	}

	// status (no changes yet)
	statusOut := captureJSON(t, func() error { return runStatus() })
	if statusOut["workspace_changes"] == nil {
		t.Errorf("status: missing workspace_changes")
	}
	if statusOut["conflicts"] == nil {
		t.Errorf("status: missing conflicts")
	}

	// Modify workspace, sync.
	if err := os.WriteFile(filepath.Join(dest, "main.go"), []byte("package main\nfunc main(){println(2)}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	syncOut := captureJSON(t, func() error { return runSync(true, false, true) })
	if syncOut["applied"] != true {
		t.Errorf("sync: expected applied=true, got %+v", syncOut)
	}
	if syncOut["history_id"] == nil {
		t.Errorf("sync: missing history_id")
	}

	// history
	histOut := captureJSON(t, func() error { return runHistory() })
	entries, ok := histOut["entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Errorf("history: expected 1 entry, got %+v", histOut)
	}

	// undo
	undoOut := captureJSON(t, func() error { return runUndo(1) })
	undone, ok := undoOut["undone"].([]any)
	if !ok || len(undone) != 1 {
		t.Errorf("undo: expected 1 undone, got %+v", undoOut)
	}
}

func TestJSONSyncConflictNoForce(t *testing.T) {
	main := setupProject(t)
	if _, err := captureJSONOrErr(func() error { return runInit(false) }); err != nil {
		t.Fatal(err)
	}
	cur, _ := os.ReadFile(filepath.Join(main, ".agentignore"))
	_ = os.WriteFile(filepath.Join(main, ".agentignore"), append(cur, []byte("\n.env\n")...), 0o644)

	dest := filepath.Join(t.TempDir(), "ws")
	if _, err := captureJSONOrErr(func() error { return runCopy(dest, false) }); err != nil {
		t.Fatal(err)
	}

	// Cause a conflict.
	if err := os.WriteFile(filepath.Join(dest, "main.go"), []byte("ws\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "main.go"), []byte("drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := captureJSON(t, func() error { return runSync(true, false, true) })
	if out["applied"] != false {
		t.Errorf("conflict sync: expected applied=false, got %+v", out)
	}
	conflicts, _ := out["conflicts"].([]any)
	if len(conflicts) != 1 {
		t.Errorf("conflict sync: expected 1 conflict, got %+v", out["conflicts"])
	}
}

func TestReset_JSONShape(t *testing.T) {
	main := setupProject(t)

	if _, err := captureJSONOrErr(func() error { return runInit(false) }); err != nil {
		t.Fatal(err)
	}
	cur, _ := os.ReadFile(filepath.Join(main, ".agentignore"))
	_ = os.WriteFile(filepath.Join(main, ".agentignore"), append(cur, []byte("\n.env\n")...), 0o644)

	dest := filepath.Join(t.TempDir(), "ws")
	if _, err := captureJSONOrErr(func() error { return runCopy(dest, false) }); err != nil {
		t.Fatal(err)
	}

	// produce a sync so history exists
	if err := os.WriteFile(filepath.Join(dest, "main.go"), []byte("package main\nfunc main(){println(2)}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := captureJSONOrErr(func() error { return runSync(true, false, true) }); err != nil {
		t.Fatal(err)
	}

	// reset without --with-history
	out := captureJSON(t, func() error { return runReset(true, false) })
	for _, k := range []string{"workspace_deleted", "workspace_path", "baseline_deleted", "history_deleted"} {
		if _, ok := out[k]; !ok {
			t.Errorf("reset: missing key %q in %+v", k, out)
		}
	}
	if out["workspace_deleted"] != true {
		t.Errorf("reset: expected workspace_deleted=true, got %+v", out)
	}
	if out["baseline_deleted"] != true {
		t.Errorf("reset: expected baseline_deleted=true, got %+v", out)
	}
	if out["history_deleted"] != false {
		t.Errorf("reset: expected history_deleted=false without --with-history, got %+v", out)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("workspace dir should be deleted")
	}
	// history dir should still exist
	if _, err := os.Stat(filepath.Join(main, ".agent", "history")); err != nil {
		t.Errorf("history dir should be preserved without --with-history: %v", err)
	}
	// main project files must still exist (security invariant)
	if _, err := os.Stat(filepath.Join(main, "main.go")); err != nil {
		t.Errorf("main project file must not be touched by reset: %v", err)
	}
	if _, err := os.Stat(filepath.Join(main, ".env")); err != nil {
		t.Errorf("ignored main file must not be touched by reset: %v", err)
	}

	// reset --with-history: re-copy first since workspace was cleared
	dest2 := filepath.Join(t.TempDir(), "ws2")
	if _, err := captureJSONOrErr(func() error { return runCopy(dest2, false) }); err != nil {
		t.Fatal(err)
	}
	out2 := captureJSON(t, func() error { return runReset(true, true) })
	if out2["history_deleted"] != true {
		t.Errorf("reset --with-history: expected history_deleted=true, got %+v", out2)
	}
	if _, err := os.Stat(filepath.Join(main, ".agent", "history")); !os.IsNotExist(err) {
		t.Errorf("history dir should be deleted with --with-history")
	}
}

func TestMask_Lifecycle_JSONShape(t *testing.T) {
	main := setupProject(t)

	if _, err := captureJSONOrErr(func() error { return runInit(false) }); err != nil {
		t.Fatal(err)
	}
	cur, _ := os.ReadFile(filepath.Join(main, ".agentignore"))
	_ = os.WriteFile(filepath.Join(main, ".agentignore"), append(cur, []byte("\n.env\n")...), 0o644)

	// Create a yaml file with a secret.
	appYAML := "pw: realpw123\nname: app1\n"
	if err := os.WriteFile(filepath.Join(main, "app.yaml"), []byte(appYAML), 0o644); err != nil {
		t.Fatal(err)
	}

	// mask add
	addOut := captureJSON(t, func() error { return runMaskAdd("app.yaml", []string{"pw"}) })
	if addOut["added"] != true {
		t.Errorf("mask add: expected added=true, got %+v", addOut)
	}
	if addOut["file"] != "app.yaml" {
		t.Errorf("mask add: expected file=app.yaml, got %+v", addOut["file"])
	}
	if n, _ := addOut["total_masks"].(float64); int(n) != 1 {
		t.Errorf("mask add: expected total_masks=1, got %+v", addOut["total_masks"])
	}

	// mask list
	listOut := captureJSON(t, func() error { return runMaskList() })
	masks, ok := listOut["masks"].([]any)
	if !ok || len(masks) != 1 {
		t.Errorf("mask list: expected 1 mask, got %+v", listOut)
	}

	// copy
	dest := filepath.Join(t.TempDir(), "ws")
	if _, err := captureJSONOrErr(func() error { return runCopy(dest, false) }); err != nil {
		t.Fatal(err)
	}

	// Workspace yaml should show placeholder.
	wsYAML, err := os.ReadFile(filepath.Join(dest, "app.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(wsYAML), "***MASKED***") {
		t.Errorf("workspace app.yaml should contain placeholder, got: %s", wsYAML)
	}
	if contains(string(wsYAML), "realpw123") {
		t.Errorf("workspace app.yaml must NOT leak real value, got: %s", wsYAML)
	}
	// Security invariant: main file is untouched.
	mainYAML, _ := os.ReadFile(filepath.Join(main, "app.yaml"))
	if !contains(string(mainYAML), "realpw123") {
		t.Errorf("main app.yaml must still have real value, got: %s", mainYAML)
	}

	// Agent edits: tries to overwrite masked path AND renames name.
	tampered := "pw: hacked\nname: app2\n"
	if err := os.WriteFile(filepath.Join(dest, "app.yaml"), []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}

	statusOut := captureJSON(t, func() error { return runStatus() })
	wsChanges, _ := statusOut["workspace_changes"].([]any)
	if len(wsChanges) != 1 {
		t.Errorf("status: expected 1 change (app.yaml name field), got %+v", statusOut)
	}

	// sync
	syncOut := captureJSON(t, func() error { return runSync(true, false, true) })
	if syncOut["applied"] != true {
		t.Errorf("sync: expected applied=true, got %+v", syncOut)
	}

	// Security invariant: main's pw is still realpw123, but name was updated.
	mainAfter, _ := os.ReadFile(filepath.Join(main, "app.yaml"))
	if !contains(string(mainAfter), "realpw123") {
		t.Errorf("after sync, main pw must still be realpw123 (agent's 'hacked' must be discarded), got: %s", mainAfter)
	}
	if contains(string(mainAfter), "hacked") {
		t.Errorf("after sync, main must NOT contain 'hacked' value, got: %s", mainAfter)
	}
	if !contains(string(mainAfter), "app2") {
		t.Errorf("after sync, main name must be app2, got: %s", mainAfter)
	}

	// mask rm
	rmOut := captureJSON(t, func() error { return runMaskRm("app.yaml", "") })
	if n, _ := rmOut["remaining"].(float64); int(n) != 0 {
		t.Errorf("mask rm: expected remaining=0, got %+v", rmOut)
	}
	removed, _ := rmOut["removed"].([]any)
	if len(removed) != 1 {
		t.Errorf("mask rm: expected 1 removed path, got %+v", rmOut)
	}
}

func TestMask_RejectsNonYamlJson(t *testing.T) {
	main := setupProject(t)
	if _, err := captureJSONOrErr(func() error { return runInit(false) }); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "notes.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "config.toml"), []byte("k=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := captureJSONOrErr(func() error { return runMaskAdd("notes.txt", []string{"foo"}) }); err == nil {
		t.Errorf("expected error masking .txt file")
	}
	if _, err := captureJSONOrErr(func() error { return runMaskAdd("config.toml", []string{"k"}) }); err == nil {
		t.Errorf("expected error masking .toml file")
	}
}

func TestMask_NonexistentPath(t *testing.T) {
	main := setupProject(t)
	if _, err := captureJSONOrErr(func() error { return runInit(false) }); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(main, "app.yaml"), []byte("foo: bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := captureJSONOrErr(func() error { return runMaskAdd("app.yaml", []string{"nonexistent.path"}) }); err == nil {
		t.Errorf("expected error for nonexistent path")
	}
	// File must not be modified by failed add.
	got, _ := os.ReadFile(filepath.Join(main, "app.yaml"))
	if string(got) != "foo: bar\n" {
		t.Errorf("main file must not be touched on error, got: %s", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// captureJSONOrErr is like captureJSON but returns error instead of failing.
func captureJSONOrErr(fn func() error) (map[string]any, error) {
	jsonOutput = true
	defer func() { jsonOutput = false }()
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	orig := os.Stdout
	os.Stdout = w
	errCh := make(chan error, 1)
	go func() { errCh <- fn() }()
	doneCh := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(r)
		doneCh <- data
	}()
	runErr := <-errCh
	w.Close()
	os.Stdout = orig
	data := <-doneCh
	if runErr != nil {
		return nil, runErr
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
