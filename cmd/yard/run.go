package main

import (
	"fmt"

	"github.com/ponchione/sodoryard/internal/cmdutil"
	"github.com/spf13/cobra"
)

const (
	yardRunExitOK             = cmdutil.HeadlessExitOK
	yardRunExitInfrastructure = cmdutil.HeadlessExitInfrastructure
	yardRunExitSafetyLimit    = cmdutil.HeadlessExitSafetyLimit
	yardRunExitEscalation     = cmdutil.HeadlessExitEscalation
)

type yardRunExitError struct {
	code int
	err  error
}

func (e yardRunExitError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e yardRunExitError) Unwrap() error { return e.err }
func (e yardRunExitError) ExitCode() int { return e.code }

type yardRunFlags = cmdutil.HeadlessRunFlags
type yardRunResult = cmdutil.HeadlessRunResult

func newYardRunCmd(configPath *string) *cobra.Command {
	flags := yardRunFlags{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run one autonomous headless agent session",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := yardRunHeadless(cmd, *configPath, flags)
			if result != nil && (result.ExitCode == yardRunExitOK || result.ExitCode == yardRunExitSafetyLimit) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), result.ReceiptPath)
			}
			if err != nil {
				return err
			}
			if result != nil && result.ExitCode != yardRunExitOK {
				return yardRunExitError{code: result.ExitCode, err: fmt.Errorf("headless run exited with code %d", result.ExitCode)}
			}
			return nil
		},
	}
	cmdutil.RegisterHeadlessRunFlags(cmd.Flags(), &flags)
	return cmd
}

func yardRunHeadless(cmd *cobra.Command, configPath string, flags yardRunFlags) (*yardRunResult, error) {
	result, err := cmdutil.RunHeadless(cmd.Context(), cmd.ErrOrStderr(), configPath, flags)
	if err != nil {
		return nil, yardRunExitError{code: yardRunExitInfrastructure, err: err}
	}
	return result, nil
}
