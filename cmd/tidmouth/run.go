package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ponchione/sodoryard/internal/headless"
)

const (
	runExitOK             = int(headless.ExitOK)
	runExitInfrastructure = int(headless.ExitInfrastructure)
	runExitSafetyLimit    = int(headless.ExitSafetyLimit)
	runExitEscalation     = int(headless.ExitEscalation)
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

type runFlags struct {
	Role        string
	Task        string
	TaskFile    string
	ChainID     string
	Brain       string
	MaxTurns    int
	MaxTokens   int
	Timeout     time.Duration
	ReceiptPath string
	Quiet       bool
	ProjectRoot string
}

type runExecutionResult struct {
	ReceiptPath string
	ExitCode    int
}

func newRunCmd(configPath *string) *cobra.Command {
	flags := runFlags{Timeout: 30 * time.Minute}
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
	cmd.Flags().StringVar(&flags.Role, "role", "", "Agent role from config")
	cmd.Flags().StringVar(&flags.Task, "task", "", "Task text for the headless run")
	cmd.Flags().StringVar(&flags.TaskFile, "task-file", "", "Read task text from file")
	cmd.Flags().StringVar(&flags.ChainID, "chain-id", "", "Chain execution identifier")
	cmd.Flags().StringVar(&flags.Brain, "brain", "", "Override brain vault path")
	cmd.Flags().IntVar(&flags.MaxTurns, "max-turns", 0, "Override max turns for this run")
	cmd.Flags().IntVar(&flags.MaxTokens, "max-tokens", 0, "Override max total tokens for this run")
	cmd.Flags().DurationVar(&flags.Timeout, "timeout", 30*time.Minute, "Wall-clock timeout for the entire session")
	cmd.Flags().StringVar(&flags.ReceiptPath, "receipt-path", "", "Override brain-relative receipt path")
	cmd.Flags().BoolVar(&flags.Quiet, "quiet", false, "Suppress progress output")
	cmd.Flags().StringVar(&flags.ProjectRoot, "project-root", "", "Override project root")
	return cmd
}

func runHeadless(cmd *cobra.Command, configPath string, flags runFlags) (*runExecutionResult, error) {
	result, err := headless.RunSession(cmd.Context(), cmd.ErrOrStderr(), configPath, headless.RunRequest{
		Role:        flags.Role,
		Task:        flags.Task,
		TaskFile:    flags.TaskFile,
		ChainID:     flags.ChainID,
		Brain:       flags.Brain,
		MaxTurns:    flags.MaxTurns,
		MaxTokens:   flags.MaxTokens,
		Timeout:     flags.Timeout,
		ReceiptPath: flags.ReceiptPath,
		Quiet:       flags.Quiet,
		ProjectRoot: flags.ProjectRoot,
	}, headless.Deps{})
	if err != nil {
		return nil, runExitError{code: runExitInfrastructure, err: err}
	}
	return &runExecutionResult{ReceiptPath: result.ReceiptPath, ExitCode: int(result.ExitCode)}, nil
}
