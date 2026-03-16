package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/micron/micron/pkg/analysis/analyzer"
	"github.com/micron/micron/pkg/analysis/report"
	"github.com/micron/micron/pkg/core/scanner"
	"github.com/micron/micron/pkg/pipeline/compressor"
	"github.com/micron/micron/pkg/pipeline/optimizer"
)

// TestFullPipeline tests the complete scan → analyze → optimize → pack workflow
func TestFullPipeline(t *testing.T) {
	// Create test project structure
	tempDir := t.TempDir()
	projectDir := filepath.Join(tempDir, "testproject")

	// Create files that would be in a typical build output
	files := map[string]string{
		"dist/app.js":          "console.log('app');",
		"dist/app.js.map":      "{\"version\": 3}", // source map (should be optimized)
		"dist/app.css":         "body { margin: 0 }",
		"dist/app.css.map":     "{\"version\": 3}", // source map
		"dist/index.html":      "<html></html>",
		"dist/assets/logo.png": "fakepngdata",
		"dist/debug.log":       "debug output", // log file
		"dist/temp.tmp":        "temporary",    // temp file
		"dist/README.md":       "# Project",    // docs (not optimized by default)
		".git/config":          "[core]",       // should be excluded
	}

	for name, content := range files {
		path := filepath.Join(projectDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Stage 1: Scan
	sc := scanner.NewScanner()
	scanResult, err := sc.Scan(projectDir)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if scanResult.TotalFiles < len(files)-1 { // -1 for .git which might not be counted
		t.Errorf("expected at least %d files, got %d", len(files)-1, scanResult.TotalFiles)
	}

	// Stage 2: Analyze
	analysis := analyzer.Analyze(scanResult, 20)

	if len(analysis.PotentialSavings.Candidates) == 0 {
		t.Error("expected optimization candidates")
	}

	// Stage 3: Optimize (dry-run)
	opts := optimizer.Options{
		DryRun:       true,
		ExcludeGlobs: []string{},
	}

	op := optimizer.NewOptimizer()
	op.ConfigureFromOptions(opts)

	plan, err := op.BuildPlan(scanResult, analysis, opts)
	if err != nil {
		t.Fatalf("build plan failed: %v", err)
	}

	// Should find source maps, logs, temp files
	if plan.TotalFiles == 0 {
		t.Error("expected files to optimize")
	}

	// Verify .git is excluded
	for _, item := range plan.Items {
		if filepath.Base(filepath.Dir(item.Path)) == ".git" {
			t.Errorf(".git should be excluded, found: %s", item.Path)
		}
	}

	// Stage 4: Pack
	outputPath := filepath.Join(tempDir, "output.tar.zst")
	copts := compressor.Options{
		Format:       compressor.FormatZstd,
		Level:        3,
		SkipSymlinks: true,
	}

	c := compressor.NewCompressor(copts)
	compResult, err := c.CompressDirectory(projectDir, outputPath)
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}

	if compResult.FilesProcessed == 0 {
		t.Error("expected files to be compressed")
	}

	if compResult.CompressedSize == 0 {
		t.Error("expected non-zero compressed size")
	}

	// Note: ratio can be > 1 for small files due to tar/compression overhead
	if compResult.Ratio <= 0 {
		t.Errorf("unexpected compression ratio: %f", compResult.Ratio)
	}

	// Verify archive exists
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("output archive does not exist")
	}
}

// TestCrashHandling tests error conditions and edge cases
func TestCrashHandling(t *testing.T) {
	t.Run("non-existent path", func(t *testing.T) {
		sc := scanner.NewScanner()
		_, err := sc.Scan("/nonexistent/path/that/does/not/exist")
		if err == nil {
			t.Error("expected error for non-existent path")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		tempDir := t.TempDir()
		sc := scanner.NewScanner()
		result, err := sc.Scan(tempDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.TotalFiles != 0 {
			t.Errorf("expected 0 files, got %d", result.TotalFiles)
		}
	})

	t.Run("optimizer refuses dangerous root", func(t *testing.T) {
		op := optimizer.NewOptimizer()

		scanResult := &scanner.ScanResult{
			RootPath:    "/",
			FilesByType: map[scanner.FileType]int{},
			SizeByType:  map[scanner.FileType]int64{},
			Files:       []scanner.FileEntry{},
			Duplicates:  []scanner.DuplicateGroup{},
		}
		analysis := &report.AnalysisReport{
			RootPath:         "/",
			PotentialSavings: report.PotentialSavings{},
		}

		_, err := op.BuildPlan(scanResult, analysis, optimizer.Options{})
		if err == nil {
			t.Error("expected error for dangerous root path")
		}
	})

	t.Run("compressor handles missing source", func(t *testing.T) {
		c := compressor.NewCompressor(compressor.Options{})
		_, err := c.CompressDirectory("/nonexistent", "/tmp/output.tar.zst")
		if err == nil {
			t.Error("expected error for non-existent source")
		}
	})
}

// BenchmarkScan benchmarks directory scanning performance
func BenchmarkScan(b *testing.B) {
	tempDir := b.TempDir()

	// Create 1000 files
	for i := 0; i < 1000; i++ {
		path := filepath.Join(tempDir, "subdir", fmt.Sprintf("file%d.txt", i))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	sc := scanner.NewScanner()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := sc.Scan(tempDir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOptimize benchmarks optimization planning
func BenchmarkOptimize(b *testing.B) {
	// Create test data
	files := make([]scanner.FileEntry, 1000)
	for i := 0; i < 1000; i++ {
		files[i] = scanner.FileEntry{
			Path:      "/tmp/test/file" + string(rune(i)) + ".tmp",
			Name:      "file.tmp",
			Size:      1024,
			Type:      scanner.TypeTemp,
			Extension: ".tmp",
		}
	}

	scanResult := &scanner.ScanResult{
		RootPath:    "/tmp/test",
		TotalFiles:  1000,
		FilesByType: map[scanner.FileType]int{scanner.TypeTemp: 1000},
		SizeByType:  map[scanner.FileType]int64{scanner.TypeTemp: 1024000},
		Files:       files,
	}

	analysis := analyzer.Analyze(scanResult, 20)
	op := optimizer.NewOptimizer()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := op.BuildPlan(scanResult, analysis, optimizer.Options{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCompress benchmarks compression performance
func BenchmarkCompress(b *testing.B) {
	tempDir := b.TempDir()
	projectDir := filepath.Join(tempDir, "project")

	// Create files of various sizes
	sizes := []int{100, 1000, 10000, 100000}
	for i, size := range sizes {
		content := make([]byte, size)
		for j := range content {
			content[j] = byte(j % 256)
		}
		path := filepath.Join(projectDir, "file"+string(rune(i))+".bin")
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0644); err != nil {
			b.Fatal(err)
		}
	}

	outputPath := filepath.Join(tempDir, "output.tar.zst")
	c := compressor.NewCompressor(compressor.Options{Format: compressor.FormatZstd, Level: 3})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Remove previous output
		os.Remove(outputPath)
		_, err := c.CompressDirectory(projectDir, outputPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}
