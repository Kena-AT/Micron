package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/micron/micron/pkg/analyzer"
	"github.com/micron/micron/pkg/compressor"
	"github.com/micron/micron/pkg/logger"
	"github.com/micron/micron/pkg/optimizer"
	"github.com/micron/micron/pkg/scanner"
	"github.com/spf13/cobra"
)

var (
	buildSkipOptimize bool
	buildSkipPack     bool
	buildDryRun       bool
	buildYes          bool
	buildOutput       string
	buildFormat       string
	buildLevel        int
	buildExclude      []string
)

var buildCmd = &cobra.Command{
	Use:   "build [path]",
	Short: "Full pipeline: scan, optimize, and pack build artifacts",
	Long: `Build runs the complete Micron pipeline on build artifacts:
1. Scans the build artifacts and analyzes size
2. Optimizes by removing unnecessary files (if not skipped)
3. Packs the optimized artifacts into compressed archives
4. Generates a detailed report of the process`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		start := time.Now()
		path := args[0]
		log := logger.GetLogger()

		fmt.Printf("\n🔧 Micron Build Pipeline\n")
		fmt.Printf("========================================\n\n")

		// Stage 1: Scan
		fmt.Println("[1/4] Scanning...")
		log.Info("Stage 1: Scanning %s", path)

		sc := scanner.NewScanner()
		scanResult, err := sc.Scan(path)
		if err != nil {
			log.Error("Scan failed: %v", err)
			fmt.Fprintf(os.Stderr, "Error: scan failed: %v\n", err)
			os.Exit(1)
		}

		analysis := analyzer.Analyze(scanResult, 20)

		fmt.Printf("  ✓ Found %d files in %d dirs\n", scanResult.TotalFiles, scanResult.TotalDirs)
		fmt.Printf("  ✓ Total size: %s\n", scanner.FormatSize(scanResult.TotalSize))
		fmt.Printf("  ✓ Duplicates: %d groups (%s wasted)\n",
			len(scanResult.Duplicates), scanner.FormatSize(analysis.DuplicateSummary.WastedBytes))

		// Stage 2: Optimize
		var optPlan *optimizer.OptimizationPlan
		if !buildSkipOptimize {
			fmt.Println("\n[2/4] Optimizing...")
			log.Info("Stage 2: Optimizing %s", path)

			opts := optimizer.Options{
				DryRun:          buildDryRun,
				Yes:             buildYes,
				AllowLargeFiles: false,
				RemoveTests:     false,
				RemoveDocs:      false,
				RemoveExamples:  false,
				ExcludeGlobs:    buildExclude,
			}

			op := optimizer.NewOptimizer()
			op.ConfigureFromOptions(opts)

			optPlan, err = op.BuildPlan(scanResult, analysis, opts)
			if err != nil {
				log.Error("Optimization planning failed: %v", err)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if err := op.ApplyPlan(optPlan, opts, os.Stdout, os.Stdin); err != nil {
				log.Error("Optimization failed: %v", err)
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			// Re-scan after optimization for accurate packing stats
			if !buildDryRun && optPlan.TotalFiles > 0 {
				fmt.Println("\n  Re-scanning after optimization...")
				scanResult, _ = sc.Scan(path)
			}
		} else {
			fmt.Println("\n[2/4] Optimization skipped (--skip-optimize)")
			log.Info("Stage 2: Optimization skipped")
		}

		// Stage 3: Pack
		if !buildSkipPack {
			fmt.Println("\n[3/4] Packing...")
			log.Info("Stage 3: Packing %s", path)

			output := buildOutput
			if output == "" {
				ext := ".tar.zst"
				if buildFormat == "gzip" {
					ext = ".tar.gz"
				}
				output = path + ext
			}

			absOutput, err := filepath.Abs(output)
			if err != nil {
				log.Error("Failed to resolve output path: %v", err)
				os.Exit(1)
			}

			if buildDryRun {
				fmt.Printf("  [DRY RUN] Would create: %s\n", filepath.Base(absOutput))
			} else {
				copts := compressor.Options{
					Format:       compressor.Format(buildFormat),
					Level:        buildLevel,
					ExcludeGlobs: buildExclude,
					SkipSymlinks: true,
					ProgressFunc: func(current, total int64) {
						if total > 0 && current%1000000 == 0 { // Update every ~1MB
							pct := float64(current) / float64(total) * 100
							fmt.Printf("  Progress: %.1f%%\r", pct)
						}
					},
				}

				c := compressor.NewCompressor(copts)
				compResult, err := c.CompressDirectory(path, absOutput)
				if err != nil {
					log.Error("Pack failed: %v", err)
					fmt.Fprintf(os.Stderr, "Error: pack failed: %v\n", err)
					os.Exit(1)
				}

				fmt.Printf("  ✓ Created: %s\n", filepath.Base(absOutput))
				fmt.Printf("  ✓ Original: %s → Compressed: %s\n",
					compressor.FormatSize(compResult.OriginalSize),
					compressor.FormatSize(compResult.CompressedSize))
				fmt.Printf("  ✓ Ratio: %s\n", compressor.FormatRatio(compResult.Ratio))
			}
		} else {
			fmt.Println("\n[3/4] Packing skipped (--skip-pack)")
			log.Info("Stage 3: Packing skipped")
		}

		// Stage 4: Report
		fmt.Println("\n[4/4] Generating report...")
		log.Info("Stage 4: Generating report")

		duration := time.Since(start)
		fmt.Printf("\n========================================\n")
		fmt.Printf("📊 Build Pipeline Complete\n")
		fmt.Printf("========================================\n")
		fmt.Printf("  Source:     %s\n", path)
		fmt.Printf("  Duration:   %d ms\n", duration.Milliseconds())
		if optPlan != nil && !buildSkipOptimize {
			fmt.Printf("  Optimized:  %d files removed\n", optPlan.TotalFiles)
			fmt.Printf("  Saved:      %s\n", scanner.FormatSize(optPlan.TotalBytes))
		}
		fmt.Printf("========================================\n\n")

		log.Info("Build pipeline completed in %d ms", duration.Milliseconds())
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)

	buildCmd.Flags().BoolVar(&buildSkipOptimize, "skip-optimize", false, "Skip optimization stage")
	buildCmd.Flags().BoolVar(&buildSkipPack, "skip-pack", false, "Skip packing stage")
	buildCmd.Flags().BoolVar(&buildDryRun, "dry-run", false, "Preview changes without applying them")
	buildCmd.Flags().BoolVar(&buildYes, "yes", false, "Skip confirmation prompts")
	buildCmd.Flags().StringVarP(&buildOutput, "output", "o", "", "Output archive path (default: <path>.tar.zst)")
	buildCmd.Flags().StringVar(&buildFormat, "format", "zstd", "Compression format (zstd|gzip)")
	buildCmd.Flags().IntVar(&buildLevel, "level", 3, "Compression level (1-22 for zstd, 1-9 for gzip)")
	buildCmd.Flags().StringArrayVar(&buildExclude, "exclude", nil, "Exclude files by glob (repeatable)")
}
