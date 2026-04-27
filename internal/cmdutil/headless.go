package cmdutil

import (
	"context"
	"io"
	"time"

	"github.com/ponchione/sodoryard/internal/headless"
	"github.com/spf13/pflag"
)

const (
	HeadlessExitOK             = int(headless.ExitOK)
	HeadlessExitInfrastructure = int(headless.ExitInfrastructure)
	HeadlessExitSafetyLimit    = int(headless.ExitSafetyLimit)
	HeadlessExitEscalation     = int(headless.ExitEscalation)
)

type HeadlessRunFlags struct {
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

type HeadlessRunResult struct {
	ReceiptPath string
	ExitCode    int
}

func RegisterHeadlessRunFlags(flags *pflag.FlagSet, values *HeadlessRunFlags) {
	flags.StringVar(&values.Role, "role", "", "Agent role from config")
	flags.StringVar(&values.Task, "task", "", "Task text for the headless run")
	flags.StringVar(&values.TaskFile, "task-file", "", "Read task text from file")
	flags.StringVar(&values.ChainID, "chain-id", "", "Chain execution identifier")
	flags.StringVar(&values.Brain, "brain", "", "Override brain vault path")
	flags.IntVar(&values.MaxTurns, "max-turns", 0, "Override max turns for this run")
	flags.IntVar(&values.MaxTokens, "max-tokens", 0, "Override max total tokens for this run")
	flags.DurationVar(&values.Timeout, "timeout", 0, "Wall-clock timeout for the entire session; 0 uses the role/default timeout")
	flags.StringVar(&values.ReceiptPath, "receipt-path", "", "Override brain-relative receipt path")
	flags.BoolVar(&values.Quiet, "quiet", false, "Suppress progress output")
	flags.StringVar(&values.ProjectRoot, "project-root", "", "Override project root")
}

func RunHeadless(ctx context.Context, errOut io.Writer, configPath string, flags HeadlessRunFlags) (*HeadlessRunResult, error) {
	result, err := headless.RunSession(ctx, errOut, configPath, headless.RunRequest{
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
		return nil, err
	}
	return &HeadlessRunResult{ReceiptPath: result.ReceiptPath, ExitCode: int(result.ExitCode)}, nil
}
