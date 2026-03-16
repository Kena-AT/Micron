package scanner

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// TerminalFormatter formats scan results for terminal output
type TerminalFormatter struct{}

// NewTerminalFormatter creates a new terminal formatter
func NewTerminalFormatter() *TerminalFormatter {
	return &TerminalFormatter{}
}

// Format outputs scan results to the terminal
func (f *TerminalFormatter) Format(result *ScanResult, w io.Writer) error {
	fmt.Fprintf(w, "\n📁 Scan Results: %s\n", result.RootPath)
	fmt.Fprintf(w, "========================================\n\n")

	// Summary
	fmt.Fprintf(w, "📊 Summary:\n")
	fmt.Fprintf(w, "  Total Files: %d\n", result.TotalFiles)
	fmt.Fprintf(w, "  Total Directories: %d\n", result.TotalDirs)
	fmt.Fprintf(w, "  Total Size: %s\n", FormatSize(result.TotalSize))
	fmt.Fprintf(w, "  Scan Time: %d ms\n\n", result.ScanTimeMs)

	// Files by type
	if len(result.FilesByType) > 0 {
		fmt.Fprintf(w, "📑 Files by Type:\n")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  Type\tCount\tSize")
		fmt.Fprintln(tw, "  ----\t-----\t----")

		// Sort by count
		var types []struct {
			Type  FileType
			Count int
			Size  int64
		}
		for t, c := range result.FilesByType {
			types = append(types, struct {
				Type  FileType
				Count int
				Size  int64
			}{t, c, result.SizeByType[t]})
		}
		sort.Slice(types, func(i, j int) bool {
			return types[i].Count > types[j].Count
		})

		for _, t := range types {
			fmt.Fprintf(tw, "  %s\t%d\t%s\n", t.Type, t.Count, FormatSize(t.Size))
		}
		tw.Flush()
		fmt.Fprintln(w)
	}

	// Duplicates
	if len(result.Duplicates) > 0 {
		fmt.Fprintf(w, "⚠️  Duplicates Found:\n")
		totalWasted := int64(0)
		for _, dup := range result.Duplicates {
			totalWasted += dup.WastedSpace
			hashLen := min(16, len(dup.Hash))
			fmt.Fprintf(w, "  Hash: %s\n", dup.Hash[:hashLen])
			fmt.Fprintf(w, "    Size: %s\n", FormatSize(dup.Size))
			fmt.Fprintf(w, "    Count: %d (wasted: %s)\n", dup.Count, FormatSize(dup.WastedSpace))
			for _, path := range dup.Files[:min(3, len(dup.Files))] {
				fmt.Fprintf(w, "    - %s\n", path)
			}
			if len(dup.Files) > 3 {
				fmt.Fprintf(w, "    ... and %d more\n", len(dup.Files)-3)
			}
		}
		fmt.Fprintf(w, "\n💾 Total Wasted Space: %s\n", FormatSize(totalWasted))
	} else {
		fmt.Fprintf(w, "✅ No duplicates found\n")
	}

	fmt.Fprintln(w)
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// JSONFormatter formats scan results as JSON
type JSONFormatter struct {
	indent bool
}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter(indent bool) *JSONFormatter {
	return &JSONFormatter{indent: indent}
}

// Format outputs scan results as JSON
func (f *JSONFormatter) Format(result *ScanResult, w io.Writer) error {
	encoder := json.NewEncoder(w)
	if f.indent {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(result)
}

// SimpleFormatter outputs basic scan information
type SimpleFormatter struct{}

// NewSimpleFormatter creates a new simple formatter
func NewSimpleFormatter() *SimpleFormatter {
	return &SimpleFormatter{}
}

// Format outputs simple scan results
func (f *SimpleFormatter) Format(result *ScanResult, w io.Writer) error {
	fmt.Fprintf(w, "Path: %s\n", result.RootPath)
	fmt.Fprintf(w, "Files: %d\n", result.TotalFiles)
	fmt.Fprintf(w, "Size: %s\n", FormatSize(result.TotalSize))
	fmt.Fprintf(w, "Duplicates: %d groups\n", len(result.Duplicates))
	return nil
}

// GenerateRecommendations analyzes the scan result and provides optimization recommendations
type Recommendation struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Action      string `json:"action"`
}

// Analyze generates recommendations based on scan results
func (result *ScanResult) Analyze() []Recommendation {
	var recommendations []Recommendation

	// Check for temporary files
	if count := result.FilesByType[TypeTemp]; count > 0 {
		rec := Recommendation{
			Type:        "cleanup",
			Severity:    "low",
			Description: fmt.Sprintf("Found %d temporary files", count),
			Impact:      FormatSize(result.SizeByType[TypeTemp]),
			Action:      "Run 'micron optimize' to remove temporary files",
		}
		recommendations = append(recommendations, rec)
	}

	// Check for debug files
	if count := result.FilesByType[TypeDebug]; count > 0 {
		rec := Recommendation{
			Type:        "cleanup",
			Severity:    "medium",
			Description: fmt.Sprintf("Found %d debug symbol files", count),
			Impact:      FormatSize(result.SizeByType[TypeDebug]),
			Action:      "Remove debug symbols before deployment",
		}
		recommendations = append(recommendations, rec)
	}

	// Check for duplicates
	if len(result.Duplicates) > 0 {
		totalWasted := int64(0)
		for _, dup := range result.Duplicates {
			totalWasted += dup.WastedSpace
		}
		rec := Recommendation{
			Type:        "deduplication",
			Severity:    "high",
			Description: fmt.Sprintf("Found %d duplicate file groups", len(result.Duplicates)),
			Impact:      FormatSize(totalWasted),
			Action:      "Remove duplicate files to save space",
		}
		recommendations = append(recommendations, rec)
	}

	// Check for large resource files
	if size := result.SizeByType[TypeResource]; size > 100*1024*1024 {
		rec := Recommendation{
			Type:        "optimization",
			Severity:    "medium",
			Description: "Large resource files detected",
			Impact:      FormatSize(size),
			Action:      "Consider image/video compression or WebP format",
		}
		recommendations = append(recommendations, rec)
	}

	// Check for log files
	logCount := 0
	var logSize int64
	for _, file := range result.Files {
		if strings.HasSuffix(file.Name, ".log") || strings.Contains(file.Path, "/logs/") {
			logCount++
			logSize += file.Size
		}
	}
	if logCount > 10 {
		rec := Recommendation{
			Type:        "cleanup",
			Severity:    "low",
			Description: fmt.Sprintf("Found %d log files", logCount),
			Impact:      FormatSize(logSize),
			Action:      "Archive or remove old log files",
		}
		recommendations = append(recommendations, rec)
	}

	return recommendations
}

// FormatRecommendations outputs recommendations to the terminal
func FormatRecommendations(recs []Recommendation, w io.Writer) {
	if len(recs) == 0 {
		fmt.Fprintln(w, "✅ No optimization recommendations")
		return
	}

	fmt.Fprintf(w, "\n💡 Optimization Recommendations:\n")
	fmt.Fprintf(w, "================================\n\n")

	for i, rec := range recs {
		severityEmoji := "ℹ️"
		if rec.Severity == "medium" {
			severityEmoji = "⚠️"
		} else if rec.Severity == "high" {
			severityEmoji = "🔴"
		}

		fmt.Fprintf(w, "%d. %s [%s] %s\n", i+1, severityEmoji, strings.ToUpper(rec.Severity), rec.Description)
		fmt.Fprintf(w, "   Impact: %s\n", rec.Impact)
		fmt.Fprintf(w, "   Action: %s\n\n", rec.Action)
	}
}
