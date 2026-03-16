package cmd

import (
	"fmt"
	"os"

	"github.com/micron/micron/pkg/logger"
	"github.com/micron/micron/pkg/scanner"
	"github.com/spf13/cobra"
)

var (
	scanJSON   bool
	scanQuick  bool
	scanNoHash bool
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan build artifacts and analyze their size",
	Long: `Scan analyzes build artifacts at the specified path and provides a detailed
report of file sizes, identifying potential optimization opportunities
such as unnecessary files, duplicates, and optimization candidates.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := args[0]
		log := logger.GetLogger()

		log.Info("Starting scan of: %s", path)

		sc := scanner.NewScanner()

		if scanNoHash {
			sc.SetMinHashSize(int64(^uint64(0) >> 1)) // Max int64
		}

		var result *scanner.ScanResult
		var err error

		if scanQuick {
			result, err = sc.QuickScan(path)
		} else {
			result, err = sc.Scan(path)
		}

		if err != nil {
			log.Error("Scan failed: %v", err)
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if scanJSON {
			formatter := scanner.NewJSONFormatter(true)
			if err := formatter.Format(result, os.Stdout); err != nil {
				log.Error("Failed to format output: %v", err)
				os.Exit(1)
			}
		} else {
			formatter := scanner.NewTerminalFormatter()
			if err := formatter.Format(result, os.Stdout); err != nil {
				log.Error("Failed to format output: %v", err)
				os.Exit(1)
			}

			// Show recommendations
			recs := result.Analyze()
			scanner.FormatRecommendations(recs, os.Stdout)
		}

		log.Info("Scan completed in %d ms", result.ScanTimeMs)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
	scanCmd.Flags().BoolVar(&scanJSON, "json", false, "Output results in JSON format")
	scanCmd.Flags().BoolVar(&scanQuick, "quick", false, "Quick scan without duplicate detection")
	scanCmd.Flags().BoolVar(&scanNoHash, "no-hash", false, "Skip file hashing for faster scanning")
}
