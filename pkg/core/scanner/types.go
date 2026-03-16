package scanner

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileType represents the category of a file
type FileType string

const (
	TypeBinary    FileType = "binary"
	TypeSource    FileType = "source"
	TypeResource  FileType = "resource"
	TypeDocument  FileType = "document"
	TypeConfig    FileType = "config"
	TypeTemp      FileType = "temp"
	TypeDebug     FileType = "debug"
	TypeDuplicate FileType = "duplicate"
	TypeSymlink   FileType = "symlink"
	TypeUnknown   FileType = "unknown"
)

// FileEntry represents metadata for a single file
type FileEntry struct {
	Path         string   `json:"path"`
	Name         string   `json:"name"`
	Size         int64    `json:"size"`
	Type         FileType `json:"type"`
	Extension    string   `json:"extension"`
	ModTime      int64    `json:"mod_time"`
	IsDir        bool     `json:"is_dir"`
	Hash         string   `json:"hash,omitempty"`
	HashComputed bool     `json:"hash_computed"`
	IsDuplicate  bool     `json:"is_duplicate"`
	OriginalPath string   `json:"original_path,omitempty"`
}

// DirectoryEntry represents metadata for a directory
type DirectoryEntry struct {
	Path           string           `json:"path"`
	Name           string           `json:"name"`
	FileCount      int              `json:"file_count"`
	DirCount       int              `json:"dir_count"`
	TotalSize      int64            `json:"total_size"`
	Files          []FileEntry      `json:"files,omitempty"`
	Subdirectories []DirectoryEntry `json:"subdirectories,omitempty"`
}

// ScanResult contains the complete scan results
type ScanResult struct {
	RootPath    string             `json:"root_path"`
	TotalFiles  int                `json:"total_files"`
	TotalDirs   int                `json:"total_dirs"`
	TotalSize   int64              `json:"total_size"`
	FilesByType map[FileType]int   `json:"files_by_type"`
	SizeByType  map[FileType]int64 `json:"size_by_type"`
	Files       []FileEntry        `json:"files"`
	Duplicates  []DuplicateGroup   `json:"duplicates"`
	Directories []DirectoryEntry   `json:"directories"`
	ScanTimeMs  int64              `json:"scan_time_ms"`
}

// DuplicateGroup represents a group of duplicate files
type DuplicateGroup struct {
	Hash        string   `json:"hash"`
	Size        int64    `json:"size"`
	Count       int      `json:"count"`
	Files       []string `json:"files"`
	WastedSpace int64    `json:"wasted_space"`
}

// NewFileEntry creates a new FileEntry from file info
func NewFileEntry(path string, info os.FileInfo) *FileEntry {
	ext := filepath.Ext(info.Name())
	fileType := DetectFileType(ext, info.Name())
	if info.Mode()&os.ModeSymlink != 0 {
		fileType = TypeSymlink
	}
	return &FileEntry{
		Path:         path,
		Name:         info.Name(),
		Size:         info.Size(),
		Extension:    ext,
		ModTime:      info.ModTime().Unix(),
		IsDir:        info.IsDir(),
		HashComputed: false,
		Type:         fileType,
	}
}

// ComputeHash calculates MD5 hash of the file
func (f *FileEntry) ComputeHash() error {
	if f.IsDir {
		return fmt.Errorf("cannot hash directory")
	}
	if f.Type == TypeSymlink {
		return fmt.Errorf("cannot hash symlink")
	}

	file, err := os.Open(f.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	f.Hash = hex.EncodeToString(hash.Sum(nil))
	f.HashComputed = true
	return nil
}

// DetectFileType determines the file type based on extension and name
func DetectFileType(ext, name string) FileType {
	switch ext {
	case ".exe", ".dll", ".so", ".dylib", ".bin":
		return TypeBinary
	case ".go", ".py", ".js", ".ts", ".java", ".cpp", ".c", ".h", ".rs":
		return TypeSource
	case ".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico", ".webp":
		return TypeResource
	case ".md", ".txt", ".pdf", ".doc", ".docx":
		return TypeDocument
	case ".json", ".yaml", ".yml", ".xml", ".toml", ".ini", ".conf":
		return TypeConfig
	case ".tmp", ".temp", ".cache", ".log":
		return TypeTemp
	case ".pdb", ".dbg", ".debug":
		return TypeDebug
	}

	// Check patterns in name
	if hasSuffix(name, ".min.js") || hasSuffix(name, ".min.css") {
		return TypeResource
	}

	return TypeUnknown
}

func hasSuffix(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}

// FormatSize returns human-readable size string
func FormatSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}
