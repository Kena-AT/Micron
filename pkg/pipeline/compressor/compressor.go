package compressor

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/micron/micron/pkg/core/scanner"
)

// Format represents the compression format
type Format string

const (
	FormatZstd Format = "zstd"
	FormatGzip Format = "gzip"
)

// Options configures compression behavior
type Options struct {
	Format       Format
	Level        int
	ProgressFunc func(current, total int64)
	ExcludeGlobs []string
	SkipSymlinks bool
}

// Result contains compression statistics
type Result struct {
	SourcePath     string  `json:"source_path"`
	OutputPath     string  `json:"output_path"`
	OriginalSize   int64   `json:"original_size"`
	CompressedSize int64   `json:"compressed_size"`
	Ratio          float64 `json:"ratio"`
	FilesProcessed int     `json:"files_processed"`
	DurationMs     int64   `json:"duration_ms"`
	Format         string  `json:"format"`
}

// Compressor handles streaming compression
type Compressor struct {
	opts Options
}

// NewCompressor creates a new compressor instance
func NewCompressor(opts Options) *Compressor {
	if opts.Format == "" {
		opts.Format = FormatZstd
	}
	if opts.Level <= 0 {
		opts.Level = 3
	}
	return &Compressor{opts: opts}
}

// CompressDirectory streams a directory to a tar.zstd archive
func (c *Compressor) CompressDirectory(sourceDir, outputPath string) (*Result, error) {
	start := time.Now()

	// Ensure source is a directory
	info, err := os.Stat(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to stat source: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("source must be a directory: %s", sourceDir)
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// Create compressed writer
	var compressedWriter io.WriteCloser
	switch c.opts.Format {
	case FormatZstd:
		level := zstd.EncoderLevelFromZstd(c.opts.Level)
		enc, err := zstd.NewWriter(outFile, zstd.WithEncoderLevel(level))
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd encoder: %w", err)
		}
		compressedWriter = enc
	case FormatGzip:
		gw, err := gzip.NewWriterLevel(outFile, c.opts.Level)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip writer: %w", err)
		}
		compressedWriter = gw
	default:
		return nil, fmt.Errorf("unsupported format: %s", c.opts.Format)
	}
	defer compressedWriter.Close()

	// Create tar writer
	tw := tar.NewWriter(compressedWriter)
	defer tw.Close()

	// Track stats
	var originalSize int64
	var filesProcessed int
	var processedSize int64

	// Get total size for progress
	totalSize := c.calculateTotalSize(sourceDir)

	// Walk and compress
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip symlinks if configured
		if c.opts.SkipSymlinks && (info.Mode()&os.ModeSymlink != 0) {
			return nil
		}

		// Skip directories in tar (tar includes them implicitly)
		if info.IsDir() {
			return nil
		}

		// Check exclusions
		relPath, _ := filepath.Rel(sourceDir, path)
		if c.isExcluded(relPath) {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		// Use forward slashes for tar header names (cross-platform)
		header.Name = filepath.ToSlash(relPath)

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// Copy file content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return nil // Skip files we can't open
			}
			_, err = io.Copy(tw, file)
			file.Close()
			if err != nil {
				return err
			}
			originalSize += info.Size()
			filesProcessed++
		}

		processedSize += info.Size()
		if c.opts.ProgressFunc != nil {
			c.opts.ProgressFunc(processedSize, totalSize)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk failed: %w", err)
	}

	// Close writers to flush
	tw.Close()
	compressedWriter.Close()

	// Get compressed size
	compressedInfo, _ := outFile.Stat()
	compressedSize := compressedInfo.Size()

	duration := time.Since(start)

	// Calculate ratio
	ratio := 0.0
	if originalSize > 0 {
		ratio = float64(compressedSize) / float64(originalSize)
	}

	return &Result{
		SourcePath:     sourceDir,
		OutputPath:     outputPath,
		OriginalSize:   originalSize,
		CompressedSize: compressedSize,
		Ratio:          ratio,
		FilesProcessed: filesProcessed,
		DurationMs:     duration.Milliseconds(),
		Format:         string(c.opts.Format),
	}, nil
}

// calculateTotalSize estimates total bytes to process
func (c *Compressor) calculateTotalSize(sourceDir string) int64 {
	var total int64
	filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if c.opts.SkipSymlinks && (info.Mode()&os.ModeSymlink != 0) {
			return nil
		}
		relPath, _ := filepath.Rel(sourceDir, path)
		if c.isExcluded(relPath) {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// isExcluded checks if a path matches exclusion patterns
func (c *Compressor) isExcluded(relPath string) bool {
	// Use forward slashes for consistent matching
	relPath = filepath.ToSlash(relPath)

	// Always exclude dangerous paths
	base := filepath.Base(relPath)

	if base == ".git" || base == ".svn" || base == ".hg" || base == "node_modules" {
		return true
	}
	if base == ".DS_Store" || base == "Thumbs.db" {
		return true
	}
	if filepath.IsAbs(relPath) {
		return true
	}

	// Check if any path component is a hard-excluded directory
	parts := strings.Split(relPath, "/")
	for _, part := range parts {
		lower := strings.ToLower(part)
		if lower == ".git" || lower == ".svn" || lower == ".hg" || lower == "node_modules" {
			return true
		}
	}

	// Check user exclusions
	for _, pattern := range c.opts.ExcludeGlobs {
		matched, _ := filepath.Match(pattern, relPath)
		if matched {
			return true
		}
		// Also match against basename
		matched, _ = filepath.Match(pattern, base)
		if matched {
			return true
		}
	}
	return false
}

// FormatRatio returns a human-readable compression ratio
func FormatRatio(ratio float64) string {
	savings := (1.0 - ratio) * 100
	return fmt.Sprintf("%.1f%% (%.1f%% smaller)", ratio*100, savings)
}

// FormatSize wraps scanner.FormatSize
func FormatSize(size int64) string {
	return scanner.FormatSize(size)
}
