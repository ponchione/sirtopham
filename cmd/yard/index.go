package main

import (
	"github.com/ponchione/sodoryard/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newYardIndexCmd(configPath *string) *cobra.Command {
	return cmdutil.NewCodeIndexCommand("index", "Index the codebase for semantic retrieval", configPath, nil)
}
