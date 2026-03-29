package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:          "sirtopham",
		Short:        "A self-hosted AI coding agent",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(os.Stdout, "sirtopham %s\n", version)
			return nil
		},
	}

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the sirtopham server",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not yet implemented")
			return nil
		},
	}
	serveCmd.Flags().Bool("dev", false, "Enable development mode")

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new sirtopham project",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not yet implemented")
			return nil
		},
	}

	indexCmd := &cobra.Command{
		Use:   "index",
		Short: "Index the codebase for RAG search",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not yet implemented")
			return nil
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Show or validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not yet implemented")
			return nil
		},
	}

	rootCmd.AddCommand(serveCmd, initCmd, indexCmd, configCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
