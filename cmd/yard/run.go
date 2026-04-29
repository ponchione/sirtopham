package main

import (
	"github.com/ponchione/sodoryard/internal/cmdutil"
	"github.com/spf13/cobra"
)

const (
	yardRunExitOK             = cmdutil.HeadlessExitOK
	yardRunExitInfrastructure = cmdutil.HeadlessExitInfrastructure
	yardRunExitSafetyLimit    = cmdutil.HeadlessExitSafetyLimit
	yardRunExitEscalation     = cmdutil.HeadlessExitEscalation
)

type yardRunFlags = cmdutil.HeadlessRunFlags
type yardRunResult = cmdutil.HeadlessRunResult
type yardRunExitError = cmdutil.HeadlessExitError

func newYardRunCmd(configPath *string) *cobra.Command {
	return cmdutil.NewHeadlessRunCommand("run", "Run one autonomous headless agent session", configPath)
}

func yardRunHeadless(cmd *cobra.Command, configPath string, flags yardRunFlags) (*yardRunResult, error) {
	return cmdutil.RunHeadlessForCommand(cmd, configPath, flags)
}
