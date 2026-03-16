package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFileEntry(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		fileInfo os.FileInfo
		wantType FileType
	}{
		{
			name:     "Go source file",
			path:     "/test/main.go",
			wantType: TypeSource,
		},
		{
			name:     "Binary file",
			path:     "/test/app.exe",
			wantType: TypeBinary,
		},
		{
			name:     "Image file",
			path:     "/test/image.png",
			wantType: TypeResource,
		},
		{
			name:     "Config file",
			path:     "/test/config.json",
			wantType: TypeConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temp file for testing
			tempDir := t.TempDir()
			tempFile := filepath.Join(tempDir, filepath.Base(tt.path))
			
			if err := os.WriteFile(tempFile, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			info, err := os.Stat(tempFile)
			if err != nil {
				t.Fatalf("Failed to stat temp file: %v", err)
			}

			entry := NewFileEntry(tempFile, info)
			if entry.Type != tt.wantType {
				t.Errorf("DetectFileType() = %v, want %v", entry.Type, tt.wantType)
			}
		})
	}
}

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		ext      string
		name     string
		wantType FileType
	}{
		{".go", "main.go", TypeSource},
		{".py", "script.py", TypeSource},
		{".js", "app.js", TypeSource},
		{".exe", "app.exe", TypeBinary},
		{".dll", "lib.dll", TypeBinary},
		{".png", "image.png", TypeResource},
		{".jpg", "photo.jpg", TypeResource},
		{".json", "config.json", TypeConfig},
		{".yaml", "config.yaml", TypeConfig},
		{".tmp", "temp.tmp", TypeTemp},
		{".log", "debug.log", TypeTemp},
		{".pdb", "debug.pdb", TypeDebug},
		{".unknown", "file.unknown", TypeUnknown},
		{"", "Makefile", TypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFileType(tt.ext, tt.name)
			if got != tt.wantType {
				t.Errorf("DetectFileType(%q, %q) = %v, want %v", tt.ext, tt.name, got, tt.wantType)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1536, "1.50 KB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatSize(tt.size)
			if got != tt.want {
				t.Errorf("FormatSize(%d) = %q, want %q", tt.size, got, tt.want)
			}
		})
	}
}

func TestScannerQuickScan(t *testing.T) {
	// Create test directory structure
	tempDir := t.TempDir()
	
	// Create some test files
	files := map[string]string{
		"file1.txt": "content1",
		"file2.txt": "content2",
		"subdir/file3.txt": "content3",
	}
	
	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	scanner := NewScanner()
	result, err := scanner.QuickScan(tempDir)
	if err != nil {
		t.Fatalf("QuickScan failed: %v", err)
	}

	if result.TotalFiles != 3 {
		t.Errorf("Expected 3 files, got %d", result.TotalFiles)
	}

	if result.TotalDirs != 2 { // tempDir + subdir
		t.Errorf("Expected 2 directories, got %d", result.TotalDirs)
	}
}

func TestScannerScan(t *testing.T) {
	// Create test directory with duplicates
	tempDir := t.TempDir()
	
	// Create duplicate files
	content := []byte("duplicate content")
	files := []string{
		"file1.txt",
		"file2.txt",
		"subdir/file3.txt",
	}
	
	for _, path := range files {
		fullPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, content, 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	scanner := NewScanner()
	scanner.SetMinHashSize(1) // Hash even small files
	result, err := scanner.Scan(tempDir)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if result.TotalFiles != 3 {
		t.Errorf("Expected 3 files, got %d", result.TotalFiles)
	}

	// Should detect duplicates
	if len(result.Duplicates) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(result.Duplicates))
	}

	if len(result.Duplicates) > 0 && result.Duplicates[0].Count != 3 {
		t.Errorf("Expected 3 duplicates in group, got %d", result.Duplicates[0].Count)
	}
}

func TestScanResultAnalyze(t *testing.T) {
	result := &ScanResult{
		FilesByType: map[FileType]int{
			TypeTemp:  10,
			TypeDebug: 5,
		},
		SizeByType: map[FileType]int64{
			TypeTemp:  1024,
			TypeDebug: 2048,
		},
		Duplicates: []DuplicateGroup{
			{WastedSpace: 1000},
		},
	}

	recs := result.Analyze()
	
	if len(recs) < 2 {
		t.Errorf("Expected at least 2 recommendations, got %d", len(recs))
	}

	// Check that we have expected recommendation types
	hasTempCleanup := false
	hasDebugCleanup := false
	hasDuplicate := false
	
	for _, rec := range recs {
		switch rec.Type {
		case "cleanup":
			if rec.Description == "Found 10 temporary files" {
				hasTempCleanup = true
			}
			if rec.Description == "Found 5 debug symbol files" {
				hasDebugCleanup = true
			}
		case "deduplication":
			hasDuplicate = true
		}
	}

	if !hasTempCleanup {
		t.Error("Expected temporary file cleanup recommendation")
	}
	if !hasDebugCleanup {
		t.Error("Expected debug file cleanup recommendation")
	}
	if !hasDuplicate {
		t.Error("Expected duplicate file recommendation")
	}
}

func TestFileEntryComputeHash(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")
	
	content := []byte("test content")
	if err := os.WriteFile(tempFile, content, 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	info, err := os.Stat(tempFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	entry := NewFileEntry(tempFile, info)
	if entry.HashComputed {
		t.Error("Hash should not be computed initially")
	}

	if err := entry.ComputeHash(); err != nil {
		t.Errorf("ComputeHash failed: %v", err)
	}

	if !entry.HashComputed {
		t.Error("Hash should be marked as computed")
	}

	if entry.Hash == "" {
		t.Error("Hash should not be empty")
	}

	// Test on directory (should fail)
	dirEntry := NewFileEntry(tempDir, info)
	dirEntry.IsDir = true
	if err := dirEntry.ComputeHash(); err == nil {
		t.Error("ComputeHash should fail on directory")
	}
}

func BenchmarkScannerScan(b *testing.B) {
	tempDir := b.TempDir()
	
	// Create test files
	for i := 0; i < 100; i++ {
		path := filepath.Join(tempDir, "file", string(rune(i)), ".txt")
		os.WriteFile(path, []byte("test content"), 0644)
	}

	scanner := NewScanner()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scanner.QuickScan(tempDir)
		if err != nil {
			b.Fatalf("Scan failed: %v", err)
		}
	}
}
