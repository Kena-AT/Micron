package scanner

import (
	"bytes"
	"strings"
	"testing"
)

func TestTerminalFormatterFormat(t *testing.T) {
	result := &ScanResult{
		RootPath:    "/test",
		TotalFiles:  100,
		TotalDirs:   10,
		TotalSize:   1024 * 1024,
		ScanTimeMs:  100,
		FilesByType: map[FileType]int{
			TypeSource:   50,
			TypeResource: 30,
			TypeBinary:   20,
		},
		SizeByType: map[FileType]int64{
			TypeSource:   500 * 1024,
			TypeResource: 400 * 1024,
			TypeBinary:   124 * 1024,
		},
		Duplicates: []DuplicateGroup{
			{
				Hash:        "abc123def456",
				Size:        1024,
				Count:       3,
				Files:       []string{"/test/file1.txt", "/test/file2.txt", "/test/file3.txt"},
				WastedSpace: 2048,
			},
		},
	}

	formatter := NewTerminalFormatter()
	var buf bytes.Buffer
	
	err := formatter.Format(result, &buf)
	if err != nil {
		t.Errorf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "/test") {
		t.Error("Output should contain root path")
	}
	if !strings.Contains(output, "100") {
		t.Error("Output should contain file count")
	}
	if !strings.Contains(output, "Duplicates") {
		t.Error("Output should mention duplicates")
	}
}

func TestJSONFormatterFormat(t *testing.T) {
	result := &ScanResult{
		RootPath:   "/test",
		TotalFiles: 50,
		TotalSize:  1024,
		FilesByType: map[FileType]int{
			TypeSource: 50,
		},
		SizeByType: map[FileType]int64{
			TypeSource: 1024,
		},
	}

	formatter := NewJSONFormatter(true)
	var buf bytes.Buffer
	
	err := formatter.Format(result, &buf)
	if err != nil {
		t.Errorf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "root_path") {
		t.Error("JSON output should contain root_path")
	}
	if !strings.Contains(output, "total_files") {
		t.Error("JSON output should contain total_files")
	}
}

func TestSimpleFormatterFormat(t *testing.T) {
	result := &ScanResult{
		RootPath:   "/test",
		TotalFiles: 100,
		TotalSize:  1024 * 1024,
		Duplicates:   []DuplicateGroup{},
	}

	formatter := NewSimpleFormatter()
	var buf bytes.Buffer
	
	err := formatter.Format(result, &buf)
	if err != nil {
		t.Errorf("Format() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "/test") {
		t.Error("Output should contain path")
	}
	if !strings.Contains(output, "100") {
		t.Error("Output should contain file count")
	}
}

func TestFormatRecommendations(t *testing.T) {
	recs := []Recommendation{
		{
			Type:        "cleanup",
			Severity:    "high",
			Description: "Found 10 temp files",
			Impact:      "1 KB",
			Action:      "Remove temp files",
		},
		{
			Type:        "deduplication",
			Severity:    "medium",
			Description: "Found 5 duplicates",
			Impact:      "5 KB",
			Action:      "Remove duplicates",
		},
	}

	var buf bytes.Buffer
	FormatRecommendations(recs, &buf)

	output := buf.String()
	if !strings.Contains(output, "Optimization Recommendations") {
		t.Error("Output should contain header")
	}
	if !strings.Contains(output, "temp files") {
		t.Error("Output should contain first recommendation")
	}
	if !strings.Contains(output, "duplicates") {
		t.Error("Output should contain second recommendation")
	}
}

func TestFormatRecommendationsEmpty(t *testing.T) {
	recs := []Recommendation{}

	var buf bytes.Buffer
	FormatRecommendations(recs, &buf)

	output := buf.String()
	if !strings.Contains(output, "No optimization recommendations") {
		t.Error("Output should indicate no recommendations")
	}
}

func TestScanResultAnalyzeResourceOptimization(t *testing.T) {
	result := &ScanResult{
		FilesByType: map[FileType]int{},
		SizeByType: map[FileType]int64{
			TypeResource: 200 * 1024 * 1024, // 200 MB
		},
		Duplicates: []DuplicateGroup{},
		Files: []FileEntry{
			{Path: "/test/logs/app.log", Name: "app.log", Size: 1024},
			{Path: "/test/logs/old.log", Name: "old.log", Size: 2048},
			{Path: "/test/logs/another.log", Name: "another.log", Size: 512},
			{Path: "/test/logs/more.log", Name: "more.log", Size: 1024},
			{Path: "/test/logs/extra.log", Name: "extra.log", Size: 2048},
			{Path: "/test/logs/extra2.log", Name: "extra2.log", Size: 1024},
			{Path: "/test/logs/extra3.log", Name: "extra3.log", Size: 1024},
			{Path: "/test/logs/extra4.log", Name: "extra4.log", Size: 512},
			{Path: "/test/logs/extra5.log", Name: "extra5.log", Size: 1024},
			{Path: "/test/logs/extra6.log", Name: "extra6.log", Size: 1024},
			{Path: "/test/logs/extra7.log", Name: "extra7.log", Size: 512},
			{Path: "/test/logs/extra8.log", Name: "extra8.log", Size: 2048},
		},
	}

	recs := result.Analyze()

	hasResourceOpt := false
	hasLogCleanup := false

	for _, rec := range recs {
		if rec.Type == "optimization" && rec.Description == "Large resource files detected" {
			hasResourceOpt = true
		}
		if strings.Contains(rec.Description, "log files") {
			hasLogCleanup = true
		}
	}

	if !hasResourceOpt {
		t.Error("Expected large resource optimization recommendation")
	}
	if !hasLogCleanup {
		t.Error("Expected log file cleanup recommendation")
	}
}

func TestScanResultAnalyzeNoRecommendations(t *testing.T) {
	result := &ScanResult{
		FilesByType: map[FileType]int{},
		SizeByType:  map[FileType]int64{},
		Duplicates:  []DuplicateGroup{},
		Files:       []FileEntry{},
	}

	recs := result.Analyze()

	if len(recs) != 0 {
		t.Errorf("Expected 0 recommendations, got %d", len(recs))
	}
}

func TestScannerSetWorkers(t *testing.T) {
	s := NewScanner()
	
	// Test default workers
	if s.workers < 4 {
		t.Errorf("Expected at least 4 workers, got %d", s.workers)
	}

	// Test setting valid workers
	s.SetWorkers(8)
	if s.workers != 8 {
		t.Errorf("Expected 8 workers, got %d", s.workers)
	}

	// Test setting invalid workers
	s.SetWorkers(0)
	if s.workers != 8 { // Should keep previous value
		t.Errorf("Workers should not change with invalid value, got %d", s.workers)
	}
}

func TestScannerSetMinHashSize(t *testing.T) {
	s := NewScanner()
	
	s.SetMinHashSize(2048)
	if s.minHashSize != 2048 {
		t.Errorf("Expected minHashSize 2048, got %d", s.minHashSize)
	}
}
