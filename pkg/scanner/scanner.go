package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Scanner handles directory scanning operations
type Scanner struct {
	workers    int
	minHashSize int64
	onProgress func(current, total int64)
}

// NewScanner creates a new Scanner instance
func NewScanner() *Scanner {
	workers := runtime.NumCPU()
	if workers < 4 {
		workers = 4
	}
	return &Scanner{
		workers:     workers,
		minHashSize: 1024,
	}
}

// SetWorkers configures the number of concurrent workers
func (s *Scanner) SetWorkers(n int) {
	if n > 0 {
		s.workers = n
	}
}

// SetMinHashSize sets the minimum file size for hash computation
func (s *Scanner) SetMinHashSize(size int64) {
	s.minHashSize = size
}

// SetProgressCallback sets a callback for progress updates
func (s *Scanner) SetProgressCallback(fn func(current, total int64)) {
	s.onProgress = fn
}

// Scan performs a full directory scan
func (s *Scanner) Scan(rootPath string) (*ScanResult, error) {
	startTime := time.Now()

	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat root path: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", rootPath)
	}

	result := &ScanResult{
		RootPath:    rootPath,
		FilesByType: make(map[FileType]int),
		SizeByType:  make(map[FileType]int64),
	}

	var files []FileEntry
	var mu sync.Mutex
	var fileCount int64

	// Walk the directory tree
	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			result.TotalDirs++
			return nil
		}

		entry := NewFileEntry(path, info)
		mu.Lock()
		files = append(files, *entry)
		result.TotalSize += info.Size()
		result.FilesByType[entry.Type]++
		result.SizeByType[entry.Type] += info.Size()
		mu.Unlock()

		atomic.AddInt64(&fileCount, 1)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("directory walk failed: %w", err)
	}

	result.TotalFiles = int(fileCount)
	result.Files = files

	// Detect duplicates with lazy hashing
	s.detectDuplicates(result)

	result.ScanTimeMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// ScanWithContext performs a directory scan with cancellation support
func (s *Scanner) ScanWithContext(ctx context.Context, rootPath string) (*ScanResult, error) {
	startTime := time.Now()

	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat root path: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", rootPath)
	}

	result := &ScanResult{
		RootPath:    rootPath,
		FilesByType: make(map[FileType]int),
		SizeByType:  make(map[FileType]int64),
	}

	var files []FileEntry
	var mu sync.Mutex
	var fileCount int64

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return nil
		}

		if info.IsDir() {
			result.TotalDirs++
			return nil
		}

		entry := NewFileEntry(path, info)
		mu.Lock()
		files = append(files, *entry)
		result.TotalSize += info.Size()
		result.FilesByType[entry.Type]++
		result.SizeByType[entry.Type] += info.Size()
		mu.Unlock()

		atomic.AddInt64(&fileCount, 1)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("directory walk failed: %w", err)
	}

	result.TotalFiles = int(fileCount)
	result.Files = files

	s.detectDuplicates(result)
	result.ScanTimeMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// detectDuplicates finds duplicate files using lazy hashing
func (s *Scanner) detectDuplicates(result *ScanResult) {
	sizeMap := make(map[int64][]*FileEntry)

	// Group files by size
	for i := range result.Files {
		sizeMap[result.Files[i].Size] = append(sizeMap[result.Files[i].Size], &result.Files[i])
	}

	// Check for potential duplicates (same size)
	var duplicates []DuplicateGroup
	var mu sync.Mutex
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, s.workers)

	for size, entries := range sizeMap {
		if len(entries) < 2 {
			continue
		}

		wg.Add(1)
		go func(sz int64, ents []*FileEntry) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Skip small files for hashing
			if sz < s.minHashSize {
				return
			}

			hashMap := make(map[string][]string)

			for _, entry := range ents {
				if err := entry.ComputeHash(); err != nil {
					continue
				}
				hashMap[entry.Hash] = append(hashMap[entry.Hash], entry.Path)
			}

			for hash, paths := range hashMap {
				if len(paths) > 1 {
					wastedSpace := int64(len(paths)-1) * sz
					mu.Lock()
					duplicates = append(duplicates, DuplicateGroup{
						Hash:        hash,
						Size:        sz,
						Count:       len(paths),
						Files:       paths,
						WastedSpace: wastedSpace,
					})
					mu.Unlock()

					// Mark files as duplicates
					for i, path := range paths {
						for j := range result.Files {
							if result.Files[j].Path == path {
								result.Files[j].IsDuplicate = true
								if i > 0 {
									result.Files[j].OriginalPath = paths[0]
								}
								break
							}
						}
					}
				}
			}
		}(size, entries)
	}

	wg.Wait()
	result.Duplicates = duplicates
}

// QuickScan performs a fast scan without hashing
func (s *Scanner) QuickScan(rootPath string) (*ScanResult, error) {
	startTime := time.Now()

	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat root path: %w", err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", rootPath)
	}

	result := &ScanResult{
		RootPath:    rootPath,
		FilesByType: make(map[FileType]int),
		SizeByType:  make(map[FileType]int64),
	}

	var files []FileEntry
	var mu sync.Mutex

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			result.TotalDirs++
			return nil
		}

		entry := NewFileEntry(path, info)
		mu.Lock()
		files = append(files, *entry)
		result.TotalSize += info.Size()
		result.TotalFiles++
		result.FilesByType[entry.Type]++
		result.SizeByType[entry.Type] += info.Size()
		mu.Unlock()

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("directory walk failed: %w", err)
	}

	result.Files = files
	result.ScanTimeMs = time.Since(startTime).Milliseconds()
	return result, nil
}
