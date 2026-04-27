package main

import (
	"context"

	brainindexer "github.com/ponchione/sodoryard/internal/brain/indexer"
	brainindexstate "github.com/ponchione/sodoryard/internal/brain/indexstate"
	"github.com/ponchione/sodoryard/internal/cmdutil"
	"github.com/ponchione/sodoryard/internal/codeintel"
	"github.com/ponchione/sodoryard/internal/codeintel/embedder"
	"github.com/ponchione/sodoryard/internal/codestore"
	appconfig "github.com/ponchione/sodoryard/internal/config"
	appindex "github.com/ponchione/sodoryard/internal/index"
	rtpkg "github.com/ponchione/sodoryard/internal/runtime"
	"github.com/spf13/cobra"
)

var runIndexService = appindex.Run
var runBrainIndexCommand = runBrainIndex
var openBrainVectorStore = codestore.Open
var newBrainEmbedder = func(cfg appconfig.Embedding) codeintel.Embedder { return embedder.New(cfg) }
var buildBrainIndexBackend = rtpkg.BuildBrainBackend
var markBrainIndexFresh = brainindexstate.MarkFresh

func newIndexCmd(configPath *string) *cobra.Command {
	cmd := cmdutil.NewCodeIndexCommand("index", "Build backend retrieval indexes for internal engine use", configPath, runIndexService)
	cmd.AddCommand(newIndexBrainCmd(configPath))
	return cmd
}

func newIndexBrainCmd(configPath *string) *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "brain",
		Short: "Rebuild derived brain metadata for internal engine use",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cmdutil.LoadConfig(*configPath)
			if err != nil {
				return err
			}
			result, err := runBrainIndexCommand(cmd.Context(), cfg)
			if err != nil {
				return err
			}
			if jsonOut {
				return cmdutil.WriteJSON(cmd.OutOrStdout(), result)
			}
			cmdutil.PrintBrainIndexSummary(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit machine-readable JSON output")
	return cmd
}

func runBrainIndex(ctx context.Context, cfg *appconfig.Config) (brainindexer.Result, error) {
	return cmdutil.RunBrainIndex(ctx, cfg, brainIndexDeps())
}

func brainIndexDeps() cmdutil.BrainIndexDeps {
	return cmdutil.BrainIndexDeps{
		BuildBackend: buildBrainIndexBackend,
		OpenStore:    openBrainVectorStore,
		NewEmbedder:  newBrainEmbedder,
		MarkFresh:    markBrainIndexFresh,
	}
}
