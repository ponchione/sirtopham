package main

import (
	"fmt"

	"github.com/ponchione/sodoryard/internal/cmdutil"
	"github.com/spf13/cobra"
)

const (
	runExitOK             = cmdutil.HeadlessExitOK
	runExitInfrastructure = cmdutil.HeadlessExitInfrastructure
	runExitSafetyLimit    = cmdutil.HeadlessExitSafetyLimit
	runExitEscalation     = cmdutil.HeadlessExitEscalation
)

type runExitError struct {
	code int
	err  error
}

func (e runExitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e runExitError) Unwrap() error { return e.err }
func (e runExitError) ExitCode() int { return e.code }

type runFlags = cmdutil.HeadlessRunFlags
type runExecutionResult = cmdutil.HeadlessRunResult

func newRunCmd(configPath *string) *cobra.Command {
	flags := runFlags{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run one internal headless agent session",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := runHeadless(cmd, *configPath, flags)
			if result != nil && (result.ExitCode == runExitOK || result.ExitCode == runExitSafetyLimit) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.ReceiptPath)
			}
			if err != nil {
				return err
			}
			if result != nil && result.ExitCode != runExitOK {
				return runExitError{code: result.ExitCode, err: fmt.Errorf("headless run exited with code %d", result.ExitCode)}
			}
			return nil
		},
	}
	cmdutil.RegisterHeadlessRunFlags(cmd.Flags(), &flags)
	return cmd
}

func runHeadless(cmd *cobra.Command, configPath string, flags runFlags) (*runExecutionResult, error) {
	result, err := cmdutil.RunHeadless(cmd.Context(), cmd.ErrOrStderr(), configPath, flags)
	if err != nil {
		return nil, runExitError{code: runExitInfrastructure, err: err}
	}
	return result, nil
}
