package analyzer

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/micron/micron/pkg/analysis/report"
	"github.com/micron/micron/pkg/core/scanner"
)

const (
	ReasonTempFile      = "temp_file"
	ReasonTempDir       = "temp_dir"
	ReasonDebugArtifact = "debug_artifact"
	ReasonSourceMap     = "source_map"
	ReasonLogFile       = "log_file"
	ReasonTestArtifact  = "test_artifact"
	ReasonDoc           = "doc"
	ReasonExample       = "example"
	ReasonBuildLeftover = "build_leftover"
	ReasonDuplicate     = "duplicate"
)

// Analyze derives an AnalysisReport from a ScanResult.
// It does NOT confirm deletability; it only reports potential savings candidates.
func Analyze(scan *scanner.ScanResult, topN int) *report.AnalysisReport {
	if topN <= 0 {
		topN = 20
	}
	if topN > 1000 {
		topN = 1000
	}

	r := &report.AnalysisReport{
		RootPath:          scan.RootPath,
		OriginalSizeBytes: scan.TotalSize,
		TotalFiles:        scan.TotalFiles,
		TotalDirs:         scan.TotalDirs,
		TopN:              topN,
		LargestFiles:      nil,
		DuplicateSummary:  report.DuplicateSummary{},
		DuplicateGroups:   nil,
		PotentialSavings:  report.PotentialSavings{},
		FilesByType:       nil,
	}

	r.FilesByType = buildTypeStats(scan)
	r.LargestFiles = largestFiles(scan, topN)
	r.DuplicateGroups, r.DuplicateSummary = duplicates(scan)
	r.PotentialSavings = potentialSavings(scan, r.DuplicateSummary.WastedBytes)

	return r
}

func buildTypeStats(scan *scanner.ScanResult) []report.TypeStat {
	types := make([]report.TypeStat, 0, len(scan.FilesByType))
	for t, c := range scan.FilesByType {
		types = append(types, report.TypeStat{
			Type:      string(t),
			Count:     c,
			SizeBytes: scan.SizeByType[t],
		})
	}
	// Deterministic: sort by count desc, then size desc, then type asc
	sort.Slice(types, func(i, j int) bool {
		if types[i].Count != types[j].Count {
			return types[i].Count > types[j].Count
		}
		if types[i].SizeBytes != types[j].SizeBytes {
			return types[i].SizeBytes > types[j].SizeBytes
		}
		return types[i].Type < types[j].Type
	})
	return types
}

func largestFiles(scan *scanner.ScanResult, topN int) []report.LargestFile {
	files := make([]scanner.FileEntry, 0, len(scan.Files))
	for _, f := range scan.Files {
		files = append(files, f)
	}
	// Deterministic ordering: size desc, path asc
	sort.Slice(files, func(i, j int) bool {
		if files[i].Size != files[j].Size {
			return files[i].Size > files[j].Size
		}
		return files[i].Path < files[j].Path
	})

	if topN > len(files) {
		topN = len(files)
	}

	out := make([]report.LargestFile, 0, topN)
	for i := 0; i < topN; i++ {
		reason := classifyReason(&files[i])
		out = append(out, report.LargestFile{
			Path:      files[i].Path,
			SizeBytes: files[i].Size,
			Type:      string(files[i].Type),
			Reason:    reason,
		})
	}
	return out
}

func duplicates(scan *scanner.ScanResult) ([]report.DuplicateGroup, report.DuplicateSummary) {
	groups := make([]report.DuplicateGroup, 0, len(scan.Duplicates))
	summary := report.DuplicateSummary{}

	for _, g := range scan.Duplicates {
		paths := append([]string(nil), g.Files...)
		sort.Strings(paths) // deterministic within group

		groups = append(groups, report.DuplicateGroup{
			Hash:        g.Hash,
			SizeBytes:   g.Size,
			Count:       g.Count,
			Files:       paths,
			WastedBytes: g.WastedSpace,
		})
		summary.Groups++
		summary.Files += g.Count
		summary.WastedBytes += g.WastedSpace
	}

	// Deterministic: wasted desc, then hash asc
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].WastedBytes != groups[j].WastedBytes {
			return groups[i].WastedBytes > groups[j].WastedBytes
		}
		return groups[i].Hash < groups[j].Hash
	})

	return groups, summary
}

func potentialSavings(scan *scanner.ScanResult, duplicateWastedBytes int64) report.PotentialSavings {
	byReasonCount := map[string]int{}
	byReasonBytes := map[string]int64{}
	examples := map[string][]string{}

	candidates := make([]report.Candidate, 0)

	for _, f := range scan.Files {
		reason := classifyReason(&f)
		if reason == "" {
			continue
		}
		// Duplicate savings are counted from group wasted space, not per-file here.
		if reason == ReasonDuplicate {
			continue
		}

		byReasonCount[reason]++
		byReasonBytes[reason] += f.Size
		if len(examples[reason]) < 5 {
			examples[reason] = append(examples[reason], f.Path)
		}

		candidates = append(candidates, report.Candidate{
			Path:      f.Path,
			SizeBytes: f.Size,
			Reason:    reason,
			Type:      string(f.Type),
		})
	}

	items := make([]report.SavingsItem, 0, len(byReasonCount)+1)
	var total int64

	// Convert map to list deterministically by reason asc
	reasons := make([]string, 0, len(byReasonCount))
	for k := range byReasonCount {
		reasons = append(reasons, k)
	}
	sort.Strings(reasons)

	for _, reason := range reasons {
		items = append(items, report.SavingsItem{
			Reason:   reason,
			Files:    byReasonCount[reason],
			Bytes:    byReasonBytes[reason],
			Examples: append([]string(nil), examples[reason]...),
		})
		total += byReasonBytes[reason]
	}

	if duplicateWastedBytes > 0 {
		items = append(items, report.SavingsItem{
			Reason: ReasonDuplicate,
			Files:  0,
			Bytes:  duplicateWastedBytes,
		})
		total += duplicateWastedBytes
	}

	// Deterministic candidates order: size desc, path asc
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].SizeBytes != candidates[j].SizeBytes {
			return candidates[i].SizeBytes > candidates[j].SizeBytes
		}
		return candidates[i].Path < candidates[j].Path
	})

	return report.PotentialSavings{
		TotalBytes: total,
		Items:      items,
		Candidates: candidates,
	}
}

func classifyReason(f *scanner.FileEntry) string {
	p := filepath.ToSlash(f.Path)
	name := strings.ToLower(f.Name)
	ext := strings.ToLower(f.Extension)

	// duplicates
	if f.IsDuplicate {
		return ReasonDuplicate
	}

	// temp
	if f.Type == scanner.TypeTemp || ext == ".tmp" || ext == ".temp" || strings.Contains(p, "/cache/") {
		return ReasonTempFile
	}
	if strings.Contains(p, "/cache") {
		return ReasonTempDir
	}

	// debug + source maps
	if ext == ".map" {
		return ReasonSourceMap
	}
	if f.Type == scanner.TypeDebug || ext == ".pdb" || ext == ".dbg" || strings.Contains(name, "debug") {
		return ReasonDebugArtifact
	}

	// logs
	if ext == ".log" || strings.Contains(p, "/logs/") {
		return ReasonLogFile
	}

	// tests
	if strings.Contains(p, "/test/") || strings.Contains(p, "/__tests__/") || strings.HasPrefix(filepath.Base(p), "test") {
		return ReasonTestArtifact
	}

	// docs/examples
	if ext == ".md" {
		return ReasonDoc
	}
	if strings.Contains(p, "/examples/") {
		return ReasonExample
	}

	// leftovers
	if ext == ".bak" || strings.HasSuffix(name, ".old") {
		return ReasonBuildLeftover
	}

	return ""
}
