package main

import (
	"github.com/ponchione/sodoryard/internal/cmdutil"
	"github.com/spf13/cobra"
)

const (
	runExitOK             = cmdutil.HeadlessExitOK
	runExitInfrastructure = cmdutil.HeadlessExitInfrastructure
	runExitSafetyLimit    = cmdutil.HeadlessExitSafetyLimit
	runExitEscalation     = cmdutil.HeadlessExitEscalation
)

type runFlags = cmdutil.HeadlessRunFlags
type runExecutionResult = cmdutil.HeadlessRunResult
type runExitError = cmdutil.HeadlessExitError

func newRunCmd(configPath *string) *cobra.Command {
	return cmdutil.NewHeadlessRunCommand("run", "Run one internal headless agent session", configPath)
}

func runHeadless(cmd *cobra.Command, configPath string, flags runFlags) (*runExecutionResult, error) {
	return cmdutil.RunHeadlessForCommand(cmd, configPath, flags)
}
