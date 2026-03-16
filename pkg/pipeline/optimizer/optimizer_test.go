package optimizer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/micron/micron/pkg/analysis/analyzer"
	"github.com/micron/micron/pkg/analysis/report"
	"github.com/micron/micron/pkg/core/scanner"
)

func TestBuildPlanRespectsLargeFileSafety(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "dist")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	big := filepath.Join(root, "big.tmp")
	if err := os.WriteFile(big, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(big, DefaultMaxDeletableBytes+1); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(big)
	scan := &scanner.ScanResult{
		RootPath: root,
		Files: []scanner.FileEntry{
			{Path: big, Name: "big.tmp", Extension: ".tmp", Size: info.Size(), Type: scanner.TypeTemp},
		},
	}
	analysis := &report.AnalysisReport{
		RootPath: root,
		PotentialSavings: report.PotentialSavings{
			Candidates: []report.Candidate{
				{Path: big, SizeBytes: info.Size(), Reason: analyzer.ReasonTempFile, Type: string(scanner.TypeTemp)},
			},
		},
	}

	op := NewOptimizer()
	plan, err := op.BuildPlan(scan, analysis, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if plan.TotalFiles != 0 {
		t.Fatalf("expected 0 deletions due to large file safety, got %d", plan.TotalFiles)
	}

	plan2, err := op.BuildPlan(scan, analysis, Options{AllowLargeFiles: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan2.TotalFiles != 1 {
		t.Fatalf("expected 1 deletion with allow-large-files, got %d", plan2.TotalFiles)
	}
}

func TestApplyPlanDryRunDoesNotDelete(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "dist")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	f := filepath.Join(root, "a.tmp")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(f)

	plan := &OptimizationPlan{
		RootPath:     root,
		RulesApplied: []string{"temp_file"},
		Items:        []PlanItem{{Path: f, SizeBytes: info.Size(), Reason: analyzer.ReasonTempFile, Rule: "temp_file"}},
		TotalFiles:   1,
		TotalBytes:   info.Size(),
	}

	op := NewOptimizer()
	var out bytes.Buffer
	err := op.ApplyPlan(plan, Options{DryRun: true}, &out, bytes.NewBufferString(""))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(f); err != nil {
		t.Fatalf("expected file to still exist on dry-run")
	}
}

func TestBuildPlanRespectsExclusions(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "dist")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}

	f := filepath.Join(root, "secret.env")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(f)

	scan := &scanner.ScanResult{RootPath: root, Files: []scanner.FileEntry{{Path: f, Name: "secret.env", Extension: ".env", Size: info.Size(), Type: scanner.TypeUnknown}}}
	analysis := &report.AnalysisReport{RootPath: root, PotentialSavings: report.PotentialSavings{Candidates: []report.Candidate{{Path: f, SizeBytes: info.Size(), Reason: analyzer.ReasonBuildLeftover, Type: string(scanner.TypeUnknown)}}}}

	op := NewOptimizer()
	plan, err := op.BuildPlan(scan, analysis, Options{ExcludeGlobs: []string{"*.env"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.TotalFiles != 0 {
		t.Fatalf("expected excluded file to be skipped")
	}
}
