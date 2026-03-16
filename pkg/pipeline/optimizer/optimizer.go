package optimizer

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/micron/micron/pkg/analysis/analyzer"
	"github.com/micron/micron/pkg/analysis/report"
	"github.com/micron/micron/pkg/core/scanner"
)

const (
	DefaultMaxDeletableBytes int64 = 100 * 1024 * 1024 // 100MB
)

type Options struct {
	DryRun          bool
	Yes             bool
	AllowLargeFiles bool

	RemoveTests    bool
	RemoveDocs     bool
	RemoveExamples bool

	ExcludeGlobs []string
}

type OptimizationRule struct {
	Name        string
	Enabled     bool
	ReasonMatch string
}

type PlanItem struct {
	Path      string
	SizeBytes int64
	Reason    string
	Rule      string
}

type OptimizationPlan struct {
	RootPath string

	RulesApplied []string

	Items []PlanItem

	TotalFiles int
	TotalBytes int64
}

type Optimizer struct {
	Rules []OptimizationRule
}

func NewOptimizer() *Optimizer {
	return &Optimizer{
		Rules: DefaultRules(),
	}
}

func DefaultRules() []OptimizationRule {
	return []OptimizationRule{
		{Name: "temp_file", Enabled: true, ReasonMatch: analyzer.ReasonTempFile},
		{Name: "log_file", Enabled: true, ReasonMatch: analyzer.ReasonLogFile},
		{Name: "source_map", Enabled: true, ReasonMatch: analyzer.ReasonSourceMap},
		{Name: "debug_artifact", Enabled: true, ReasonMatch: analyzer.ReasonDebugArtifact},
		{Name: "build_leftover", Enabled: true, ReasonMatch: analyzer.ReasonBuildLeftover},

		{Name: "test_artifact", Enabled: false, ReasonMatch: analyzer.ReasonTestArtifact},
		{Name: "doc", Enabled: false, ReasonMatch: analyzer.ReasonDoc},
		{Name: "example", Enabled: false, ReasonMatch: analyzer.ReasonExample},
	}
}

func (o *Optimizer) ConfigureFromOptions(opts Options) {
	for i := range o.Rules {
		switch o.Rules[i].Name {
		case "test_artifact":
			o.Rules[i].Enabled = opts.RemoveTests
		case "doc":
			o.Rules[i].Enabled = opts.RemoveDocs
		case "example":
			o.Rules[i].Enabled = opts.RemoveExamples
		}
	}
}

func (o *Optimizer) BuildPlan(scan *scanner.ScanResult, analysis *report.AnalysisReport, opts Options) (*OptimizationPlan, error) {
	absRoot, err := filepath.Abs(filepath.Clean(scan.RootPath))
	if err != nil {
		return nil, err
	}
	if isDangerousRoot(absRoot) {
		return nil, fmt.Errorf("refusing dangerous root path: %s", absRoot)
	}

	excludeMatchers := buildExcludeMatchers(absRoot, opts.ExcludeGlobs)

	enabledRules := map[string]OptimizationRule{}
	appliedNames := make([]string, 0)
	for _, r := range o.Rules {
		if r.Enabled {
			enabledRules[r.ReasonMatch] = r
			appliedNames = append(appliedNames, r.Name)
		}
	}
	sort.Strings(appliedNames)

	items := make([]PlanItem, 0)
	var totalBytes int64

	for _, c := range analysis.PotentialSavings.Candidates {
		rule, ok := enabledRules[c.Reason]
		if !ok {
			continue
		}

		// Never optimize duplicates in V1
		if c.Reason == analyzer.ReasonDuplicate {
			continue
		}

		// Never delete symlinks
		if c.Type == string(scanner.TypeSymlink) {
			continue
		}

		candidatePath := filepath.Clean(c.Path)
		absCandidate, err := filepath.Abs(candidatePath)
		if err != nil {
			continue
		}

		if !isWithinRoot(absRoot, absCandidate) {
			continue
		}

		if isHardExcluded(absCandidate) {
			continue
		}
		if isExcluded(absCandidate, excludeMatchers) {
			continue
		}

		// Never delete dirs
		entry := findFileEntry(scan, c.Path)
		if entry != nil {
			if entry.IsDir {
				continue
			}
			if entry.Type == scanner.TypeSymlink {
				continue
			}
			if entry.Size > DefaultMaxDeletableBytes && !opts.AllowLargeFiles {
				continue
			}
		} else {
			// If we cannot find the FileEntry, be conservative.
			continue
		}

		items = append(items, PlanItem{
			Path:      absCandidate,
			SizeBytes: c.SizeBytes,
			Reason:    c.Reason,
			Rule:      rule.Name,
		})
		totalBytes += c.SizeBytes
	}

	// Deterministic ordering: size desc, path asc
	sort.Slice(items, func(i, j int) bool {
		if items[i].SizeBytes != items[j].SizeBytes {
			return items[i].SizeBytes > items[j].SizeBytes
		}
		return items[i].Path < items[j].Path
	})

	plan := &OptimizationPlan{
		RootPath:     absRoot,
		RulesApplied: appliedNames,
		Items:        items,
		TotalFiles:   len(items),
		TotalBytes:   totalBytes,
	}
	return plan, nil
}

func (o *Optimizer) ApplyPlan(plan *OptimizationPlan, opts Options, w io.Writer, in io.Reader) error {
	fmt.Fprintf(w, "\n🧹 Optimization Preview: %s\n", plan.RootPath)
	fmt.Fprintf(w, "========================================\n\n")
	fmt.Fprintf(w, "Files to delete: %d\n", plan.TotalFiles)
	fmt.Fprintf(w, "Space to save: %s\n", scanner.FormatSize(plan.TotalBytes))
	fmt.Fprintf(w, "Rules applied:\n")
	for _, r := range plan.RulesApplied {
		fmt.Fprintf(w, " - %s\n", r)
	}
	fmt.Fprintln(w)

	if plan.TotalFiles == 0 {
		fmt.Fprintln(w, "Nothing to delete.")
		return nil
	}

	// Always show a small preview
	previewN := 20
	if len(plan.Items) < previewN {
		previewN = len(plan.Items)
	}
	fmt.Fprintf(w, "Preview (top %d):\n", previewN)
	for i := 0; i < previewN; i++ {
		it := plan.Items[i]
		fmt.Fprintf(w, " - %s (%s) [%s]\n", it.Path, scanner.FormatSize(it.SizeBytes), it.Rule)
	}
	if len(plan.Items) > previewN {
		fmt.Fprintf(w, " ... and %d more\n", len(plan.Items)-previewN)
	}
	fmt.Fprintln(w)

	if opts.DryRun {
		fmt.Fprintln(w, "Dry-run enabled. No files were deleted.")
		return nil
	}

	if !opts.Yes {
		if err := confirm(plan, w, in); err != nil {
			return err
		}
	}

	// Apply deletions
	deleted := 0
	saved := int64(0)
	for _, it := range plan.Items {
		// Skip vanished files, permission issues; continue
		if err := os.Remove(it.Path); err != nil {
			continue
		}
		deleted++
		saved += it.SizeBytes
	}

	fmt.Fprintf(w, "\nDeleted %d files, saved %s\n", deleted, scanner.FormatSize(saved))
	return nil
}

func confirm(plan *OptimizationPlan, w io.Writer, in io.Reader) error {
	fmt.Fprintf(w, "Delete %d files (%s)? [y/N] ", plan.TotalFiles, scanner.FormatSize(plan.TotalBytes))
	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		return errors.New("aborted")
	}
	return nil
}

func findFileEntry(scan *scanner.ScanResult, path string) *scanner.FileEntry {
	for i := range scan.Files {
		if scan.Files[i].Path == path {
			return &scan.Files[i]
		}
	}
	return nil
}

func isWithinRoot(rootAbs, candidateAbs string) bool {
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, "..") && !strings.HasPrefix(filepath.ToSlash(rel), "../")
}

func isDangerousRoot(rootAbs string) bool {
	// block filesystem roots
	clean := filepath.Clean(rootAbs)
	if runtime.GOOS == "windows" {
		vol := filepath.VolumeName(clean)
		if vol != "" {
			// e.g. C:\
			if strings.EqualFold(clean, vol+"\\") || strings.EqualFold(clean, vol+"/") || strings.EqualFold(clean, vol) {
				return true
			}
		}
		// also block UNC root like \\server\share\
		if strings.HasPrefix(clean, "\\\\") {
			parts := strings.Split(strings.TrimPrefix(clean, "\\\\"), "\\")
			if len(parts) <= 2 {
				return true
			}
		}
	} else {
		if clean == "/" {
			return true
		}
	}

	// block user home
	if home, err := os.UserHomeDir(); err == nil {
		homeAbs, _ := filepath.Abs(filepath.Clean(home))
		if strings.EqualFold(homeAbs, clean) {
			return true
		}
	}
	return false
}

func buildExcludeMatchers(rootAbs string, globs []string) []func(string) bool {
	out := make([]func(string) bool, 0, len(globs))
	for _, g := range globs {
		glob := g
		out = append(out, func(absPath string) bool {
			rel, err := filepath.Rel(rootAbs, absPath)
			if err != nil {
				return false
			}
			rel = filepath.ToSlash(rel)
			matched, _ := filepath.Match(glob, rel)
			if matched {
				return true
			}
			// also try basename match
			base := filepath.Base(rel)
			matched, _ = filepath.Match(glob, base)
			return matched
		})
	}
	return out
}

func isExcluded(absPath string, matchers []func(string) bool) bool {
	for _, m := range matchers {
		if m(absPath) {
			return true
		}
	}
	return false
}

func isHardExcluded(absPath string) bool {
	p := filepath.ToSlash(absPath)
	lower := strings.ToLower(p)

	if strings.Contains(lower, "/.git/") || strings.HasSuffix(lower, "/.git") {
		return true
	}
	if strings.Contains(lower, "/.svn/") || strings.HasSuffix(lower, "/.svn") {
		return true
	}
	if strings.Contains(lower, "/.hg/") || strings.HasSuffix(lower, "/.hg") {
		return true
	}
	if strings.Contains(lower, "/node_modules/") || strings.HasSuffix(lower, "/node_modules") {
		return true
	}

	base := strings.ToLower(filepath.Base(absPath))
	if base == ".ds_store" || base == "thumbs.db" {
		return true
	}

	return false
}
