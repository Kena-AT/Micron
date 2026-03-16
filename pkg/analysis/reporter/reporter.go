package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"github.com/micron/micron/pkg/analysis/report"
	"github.com/micron/micron/pkg/core/scanner"
)

type TerminalReporter struct{}

type JSONReporter struct {
	Indent bool
}

func NewTerminalReporter() *TerminalReporter {
	return &TerminalReporter{}
}

func NewJSONReporter(indent bool) *JSONReporter {
	return &JSONReporter{Indent: indent}
}

func (r *TerminalReporter) PrintAnalysis(rep *report.AnalysisReport, w io.Writer) error {
	fmt.Fprintf(w, "\n🧠 Analysis Report: %s\n", rep.RootPath)
	fmt.Fprintf(w, "========================================\n\n")

	fmt.Fprintf(w, "📊 Summary:\n")
	fmt.Fprintf(w, "  Original Size: %s\n", scanner.FormatSize(rep.OriginalSizeBytes))
	fmt.Fprintf(w, "  Files: %d\n", rep.TotalFiles)
	fmt.Fprintf(w, "  Directories: %d\n\n", rep.TotalDirs)

	fmt.Fprintf(w, "💰 Potential Savings: %s\n\n", scanner.FormatSize(rep.PotentialSavings.TotalBytes))

	if len(rep.PotentialSavings.Items) > 0 {
		fmt.Fprintf(w, "Potential Savings by Reason:\n")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  Reason\tFiles\tBytes")
		fmt.Fprintln(tw, "  ------\t-----\t-----")

		items := append([]report.SavingsItem(nil), rep.PotentialSavings.Items...)
		// deterministic: bytes desc then reason asc
		sort.Slice(items, func(i, j int) bool {
			if items[i].Bytes != items[j].Bytes {
				return items[i].Bytes > items[j].Bytes
			}
			return items[i].Reason < items[j].Reason
		})

		for _, it := range items {
			fmt.Fprintf(tw, "  %s\t%d\t%s\n", it.Reason, it.Files, scanner.FormatSize(it.Bytes))
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "⚠️  Duplicates:\n")
	fmt.Fprintf(w, "  Groups: %d\n", rep.DuplicateSummary.Groups)
	fmt.Fprintf(w, "  Files: %d\n", rep.DuplicateSummary.Files)
	fmt.Fprintf(w, "  Wasted Space: %s\n\n", scanner.FormatSize(rep.DuplicateSummary.WastedBytes))

	if len(rep.LargestFiles) > 0 {
		fmt.Fprintf(w, "📦 Largest Files (Top %d):\n", rep.TopN)
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  Size\tType\tReason\tPath")
		fmt.Fprintln(tw, "  ----\t----\t------\t----")
		for _, lf := range rep.LargestFiles {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", scanner.FormatSize(lf.SizeBytes), lf.Type, lf.Reason, lf.Path)
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	return nil
}

func (r *JSONReporter) PrintAnalysis(rep *report.AnalysisReport, w io.Writer) error {
	enc := json.NewEncoder(w)
	if r.Indent {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(rep)
}
