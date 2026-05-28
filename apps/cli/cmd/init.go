package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agentroom/agentroom/pkg/logx"
	"github.com/agentroom/agentroom/pkg/workspace"
	"github.com/spf13/cobra"
)

const defaultAgentignore = `# agentroom ignore (gitignore-compatible)
# Files matched here are excluded from the workspace copy
# AND are never touched by sync.

.env
.env.*
!.env.example
*.pem
*.key
*.crt
secrets/
**/secrets/**
`

type initResult struct {
	Initialized bool   `json:"initialized"`
	AgentDir    string `json:"agent_dir"`
	IgnorePath  string `json:"ignore_path"`
	ConfigPath  string `json:"config_path"`
	IgnoreWrote bool   `json:"ignore_wrote"`
	ConfigWrote bool   `json:"config_wrote"`
}

func newInitCmd() *cobra.Command {
	var force bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Initialize agentroom in the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(force)
		},
	}
	c.Flags().BoolVar(&force, "force", false, "overwrite existing .agentignore / config")
	return c
}

func runInit(force bool) error {
	mainPath, err := os.Getwd()
	if err != nil {
		return err
	}

	res := initResult{
		AgentDir:   workspace.AgentPath(mainPath),
		IgnorePath: workspace.IgnorePath(mainPath),
		ConfigPath: workspace.ConfigPath(mainPath),
	}

	ignorePath := res.IgnorePath
	if _, err := os.Stat(ignorePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.WriteFile(ignorePath, []byte(defaultAgentignore), 0o644); err != nil {
			return err
		}
		res.IgnoreWrote = true
		if !jsonEnabled() {
			logx.Infof("created %s", ignorePath)
		}
	} else if force {
		if err := os.WriteFile(ignorePath, []byte(defaultAgentignore), 0o644); err != nil {
			return err
		}
		res.IgnoreWrote = true
		if !jsonEnabled() {
			logx.Infof("overwrote %s", ignorePath)
		}
	} else if !jsonEnabled() {
		logx.Infof("kept existing %s", ignorePath)
	}

	cfgPath := res.ConfigPath
	if _, err := os.Stat(cfgPath); err == nil && !force {
		res.Initialized = true
		if !jsonEnabled() {
			logx.Infof("kept existing %s", cfgPath)
		}
		if jsonEnabled() {
			return outputJSON(res)
		}
		return nil
	}

	cfg := &workspace.Config{
		Version:     workspace.ConfigVersion,
		ProjectName: filepath.Base(mainPath),
		MainPath:    mainPath,
		CreatedAt:   time.Now().UTC(),
	}
	if err := workspace.SaveConfig(mainPath, cfg); err != nil {
		return err
	}
	gi := []byte("# agentroom internals\nhistory/\nbaseline.json\n")
	if err := os.WriteFile(filepath.Join(workspace.AgentPath(mainPath), ".gitignore"), gi, 0o644); err != nil {
		return err
	}
	res.Initialized = true
	res.ConfigWrote = true

	if jsonEnabled() {
		return outputJSON(res)
	}

	logx.Infof("initialized %s", cfgPath)
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit .agentignore to list sensitive files.")
	fmt.Println("  2. Run `ar copy` to create the workspace copy.")
	return nil
}
