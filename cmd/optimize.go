package cmd

import (
	"fmt"
	"os"

	"github.com/micron/micron/pkg/analysis/analyzer"
	"github.com/micron/micron/pkg/core/logger"
	"github.com/micron/micron/pkg/core/scanner"
	"github.com/micron/micron/pkg/pipeline/optimizer"
	"github.com/spf13/cobra"
)

var (
	optDryRun          bool
	optYes             bool
	optAllowLargeFiles bool
	optRemoveTests     bool
	optRemoveDocs      bool
	optRemoveExamples  bool
	optExclude         []string
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize [path]",
	Short: "Optimize build artifacts by removing unnecessary files",
	Long: `Optimize analyzes the build artifacts and safely removes unnecessary files
such as debug symbols, temporary files, and other non-essential artifacts
to reduce the overall size while maintaining functionality.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		log := logger.GetLogger()

		log.Info("Starting optimization of: %s", path)

		sc := scanner.NewScanner()
		scanResult, err := sc.Scan(path)
		if err != nil {
			log.Error("Scan failed: %v", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		analysis := analyzer.Analyze(scanResult, 20)

		opts := optimizer.Options{
			DryRun:          optDryRun,
			Yes:             optYes,
			AllowLargeFiles: optAllowLargeFiles,
			RemoveTests:     optRemoveTests,
			RemoveDocs:      optRemoveDocs,
			RemoveExamples:  optRemoveExamples,
			ExcludeGlobs:    optExclude,
		}

		op := optimizer.NewOptimizer()
		op.ConfigureFromOptions(opts)

		plan, err := op.BuildPlan(scanResult, analysis, opts)
		if err != nil {
			log.Error("Failed to build optimization plan: %v", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := op.ApplyPlan(plan, opts, os.Stdout, os.Stdin); err != nil {
			log.Error("Optimization aborted/failed: %v", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		log.Info("Optimization completed")
	},
}

func init() {
	rootCmd.AddCommand(optimizeCmd)
	optimizeCmd.Flags().BoolVar(&optDryRun, "dry-run", false, "Preview deletions without removing files")
	optimizeCmd.Flags().BoolVar(&optYes, "yes", false, "Skip confirmation prompt")
	optimizeCmd.Flags().BoolVar(&optAllowLargeFiles, "allow-large-files", false, "Allow deleting files larger than 100MB")
	optimizeCmd.Flags().BoolVar(&optRemoveTests, "remove-tests", false, "Include test artifacts in optimization")
	optimizeCmd.Flags().BoolVar(&optRemoveDocs, "remove-docs", false, "Include documentation files in optimization")
	optimizeCmd.Flags().BoolVar(&optRemoveExamples, "remove-examples", false, "Include examples in optimization")
	optimizeCmd.Flags().StringArrayVar(&optExclude, "exclude", nil, "Exclude files by glob (repeatable)")
}
