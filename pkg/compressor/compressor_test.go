package compressor

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestNewCompressorDefaults(t *testing.T) {
	c := NewCompressor(Options{})
	if c.opts.Format != FormatZstd {
		t.Errorf("expected default format zstd, got %s", c.opts.Format)
	}
	if c.opts.Level != 3 {
		t.Errorf("expected default level 3, got %d", c.opts.Level)
	}
}

func TestCompressDirectoryZstd(t *testing.T) {
	// Create test directory structure
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test files
	files := map[string]string{
		"file1.txt":     "Hello, World!",
		"subdir/file.js": "console.log('test');",
		"data.json":     `{"key": "value"}`,
	}
	for name, content := range files {
		path := filepath.Join(sourceDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	outputPath := filepath.Join(tempDir, "output.tar.zst")

	// Compress
	opts := Options{
		Format:     FormatZstd,
		Level:      3,
		SkipSymlinks: true,
	}
	c := NewCompressor(opts)

	result, err := c.CompressDirectory(sourceDir, outputPath)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	// Verify result
	if result.SourcePath != sourceDir {
		t.Errorf("expected source path %s, got %s", sourceDir, result.SourcePath)
	}
	if result.OutputPath != outputPath {
		t.Errorf("expected output path %s, got %s", outputPath, result.OutputPath)
	}
	if result.Format != "zstd" {
		t.Errorf("expected format zstd, got %s", result.Format)
	}
	if result.FilesProcessed != 3 {
		t.Errorf("expected 3 files, got %d", result.FilesProcessed)
	}
	if result.OriginalSize == 0 {
		t.Error("expected non-zero original size")
	}
	if result.CompressedSize == 0 {
		t.Error("expected non-zero compressed size")
	}
	if result.DurationMs < 0 {
		t.Error("expected non-negative duration")
	}

	// Verify archive exists and is readable
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("output archive does not exist")
	}

	// Verify archive contains expected files
	verifyArchiveContents(t, outputPath, FormatZstd, []string{"file1.txt", "subdir/file.js", "data.json"})
}

func TestCompressDirectoryGzip(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(tempDir, "output.tar.gz")

	opts := Options{
		Format:       FormatGzip,
		Level:        gzip.DefaultCompression,
		SkipSymlinks: true,
	}
	c := NewCompressor(opts)

	result, err := c.CompressDirectory(sourceDir, outputPath)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	if result.Format != "gzip" {
		t.Errorf("expected format gzip, got %s", result.Format)
	}

	verifyArchiveContents(t, outputPath, FormatGzip, []string{"test.txt"})
}

func TestProgressFunc(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file
	if err := os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	var progressCalled bool
	opts := Options{
		Format:     FormatZstd,
		Level:      3,
		SkipSymlinks: true,
		ProgressFunc: func(current, total int64) {
			progressCalled = true
		},
	}
	c := NewCompressor(opts)

	outputPath := filepath.Join(tempDir, "output.tar.zst")
	_, err := c.CompressDirectory(sourceDir, outputPath)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	if !progressCalled {
		t.Error("progress function was not called")
	}
}

func TestExcludeGlobs(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create files
	if err := os.WriteFile(filepath.Join(sourceDir, "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "exclude.log"), []byte("log"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Format:       FormatZstd,
		Level:        3,
		SkipSymlinks: true,
		ExcludeGlobs: []string{"*.log"},
	}
	c := NewCompressor(opts)

	outputPath := filepath.Join(tempDir, "output.tar.zst")
	result, err := c.CompressDirectory(sourceDir, outputPath)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	if result.FilesProcessed != 1 {
		t.Errorf("expected 1 file (exclude.log excluded), got %d", result.FilesProcessed)
	}

	// Verify archive contents
	verifyArchiveContents(t, outputPath, FormatZstd, []string{"keep.txt"})
}

func TestSkipSymlinks(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a file and a symlink
	if err := os.WriteFile(filepath.Join(sourceDir, "real.txt"), []byte("real"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(sourceDir, "real.txt"), filepath.Join(sourceDir, "link.txt")); err != nil {
		t.Skip("cannot create symlinks on this system")
	}

	opts := Options{
		Format:       FormatZstd,
		Level:        3,
		SkipSymlinks: true,
	}
	c := NewCompressor(opts)

	outputPath := filepath.Join(tempDir, "output.tar.zst")
	result, err := c.CompressDirectory(sourceDir, outputPath)
	if err != nil {
		t.Fatalf("compression failed: %v", err)
	}

	if result.FilesProcessed != 1 {
		t.Errorf("expected 1 file (symlink skipped), got %d", result.FilesProcessed)
	}
}

func TestFormatRatio(t *testing.T) {
	tests := []struct {
		ratio    float64
		expected string
	}{
		{0.5, "50.0% (50.0% smaller)"},
		{0.25, "25.0% (75.0% smaller)"},
		{1.0, "100.0% (0.0% smaller)"},
		{0.0, "0.0% (100.0% smaller)"},
	}

	for _, tt := range tests {
		result := FormatRatio(tt.ratio)
		if result != tt.expected {
			t.Errorf("FormatRatio(%f) = %s, expected %s", tt.ratio, result, tt.expected)
		}
	}
}

func TestIsExcluded(t *testing.T) {
	opts := Options{
		ExcludeGlobs: []string{"*.tmp", "cache/*"},
	}
	c := NewCompressor(opts)

	tests := []struct {
		path     string
		expected bool
	}{
		{"file.txt", false},
		{"file.tmp", true},
		{"cache/data.json", true},
		{".git/config", true},  // Hard excluded
		{"node_modules/pkg", true}, // Hard excluded
		{".DS_Store", true},    // Hard excluded
		{"Thumbs.db", true},    // Hard excluded
	}

	for _, tt := range tests {
		result := c.isExcluded(tt.path)
		if result != tt.expected {
			t.Errorf("isExcluded(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

// verifyArchiveContents checks that an archive contains expected entries
func verifyArchiveContents(t *testing.T, archivePath string, format Format, expected []string) {
	t.Helper()

	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer file.Close()

	var r io.Reader = file
	switch format {
	case FormatZstd:
		zr, err := zstd.NewReader(file)
		if err != nil {
			t.Fatalf("failed to create zstd reader: %v", err)
		}
		defer zr.Close()
		r = zr
	case FormatGzip:
		gr, err := gzip.NewReader(file)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer gr.Close()
		r = gr
	}

	tr := tar.NewReader(r)
	found := make(map[string]bool)
	for _, name := range expected {
		found[name] = false
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar: %v", err)
		}
		if _, ok := found[header.Name]; ok {
			found[header.Name] = true
		}
	}

	for name, wasFound := range found {
		if !wasFound {
			t.Errorf("expected entry %s not found in archive", name)
		}
	}
}
