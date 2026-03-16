package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/micron/micron/pkg/compressor"
	"github.com/micron/micron/pkg/logger"
	"github.com/spf13/cobra"
)

var (
	packFormat  string
	packLevel   int
	packOutput  string
	packExclude []string
	packDryRun  bool
)

var packCmd = &cobra.Command{
	Use:   "pack [path]",
	Short: "Compress build artifacts into optimized archives",
	Long: `Pack compresses build artifacts into efficient archives using
intelligent compression algorithms. It produces smaller distributable
artifacts while maintaining the ability to decompress them when needed.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		log := logger.GetLogger()

		log.Info("Starting pack of: %s", path)

		// Determine output path
		output := packOutput
		if output == "" {
			ext := ".tar.zst"
			if packFormat == "gzip" {
				ext = ".tar.gz"
			}
			output = path + ext
		}

		// Resolve to absolute path
		absOutput, err := filepath.Abs(output)
		if err != nil {
			log.Error("Failed to resolve output path: %v", err)
			os.Exit(1)
		}

		if packDryRun {
			fmt.Printf("[DRY RUN] Would pack: %s\n", path)
			fmt.Printf("[DRY RUN] Output: %s\n", absOutput)
			fmt.Printf("[DRY RUN] Format: %s, Level: %d\n", packFormat, packLevel)
			return
		}

		// Configure compressor
		opts := compressor.Options{
			Format:       compressor.Format(packFormat),
			Level:        packLevel,
			ExcludeGlobs: packExclude,
			SkipSymlinks: true,
			ProgressFunc: func(current, total int64) {
				if total > 0 {
					pct := float64(current) / float64(total) * 100
					fmt.Printf("\rCompressing... %.1f%%", pct)
				}
			},
		}

		c := compressor.NewCompressor(opts)

		result, err := c.CompressDirectory(path, absOutput)
		if err != nil {
			log.Error("Pack failed: %v", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Clear progress line
		fmt.Printf("\r%s\r", strings.Repeat(" ", 40))

		// Print results
		fmt.Printf("\n📦 Pack completed: %s\n", filepath.Base(absOutput))
		fmt.Printf("========================================\n")
		fmt.Printf("  Original size:  %s\n", compressor.FormatSize(result.OriginalSize))
		fmt.Printf("  Compressed:     %s\n", compressor.FormatSize(result.CompressedSize))
		fmt.Printf("  Ratio:          %s\n", compressor.FormatRatio(result.Ratio))
		fmt.Printf("  Files:          %d\n", result.FilesProcessed)
		fmt.Printf("  Duration:       %d ms\n", result.DurationMs)
		fmt.Printf("  Format:         %s\n", result.Format)
		fmt.Println()

		log.Info("Pack completed: %s", absOutput)
	},
}

func init() {
	rootCmd.AddCommand(packCmd)
	packCmd.Flags().StringVar(&packFormat, "format", "zstd", "Compression format (zstd|gzip)")
	packCmd.Flags().IntVar(&packLevel, "level", 3, "Compression level (1-22 for zstd, 1-9 for gzip)")
	packCmd.Flags().StringVarP(&packOutput, "output", "o", "", "Output archive path (default: <path>.tar.zst)")
	packCmd.Flags().StringArrayVar(&packExclude, "exclude", nil, "Exclude files by glob (repeatable)")
	packCmd.Flags().BoolVar(&packDryRun, "dry-run", false, "Show what would be packed without creating archive")
}
