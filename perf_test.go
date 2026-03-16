package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/micron/micron/pkg/core/scanner"
)

func Run() {
	// Create temp directory with many files
	tempDir, err := os.MkdirTemp("", "micron_perf_test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	// Create 100k files
	numFiles := 100000
	fmt.Printf("Creating %d files...\n", numFiles)

	for i := 0; i < numFiles; i++ {
		subdir := filepath.Join(tempDir, fmt.Sprintf("subdir%d", i%100))
		os.MkdirAll(subdir, 0755)

		filename := filepath.Join(subdir, fmt.Sprintf("file%d.txt", i))
		content := fmt.Sprintf("test content %d", i)
		os.WriteFile(filename, []byte(content), 0644)
	}

	// Scan
	fmt.Println("Scanning...")
	s := scanner.NewScanner()

	start := time.Now()
	result, err := s.QuickScan(tempDir)
	if err != nil {
		panic(err)
	}
	elapsed := time.Since(start)

	fmt.Printf("Scanned %d files in %d ms\n", result.TotalFiles, elapsed.Milliseconds())

	if elapsed > 10*time.Second {
		fmt.Println("WARNING: Scan took longer than 10 seconds!")
	} else {
		fmt.Println("✓ Performance requirement met (< 10s)")
	}
}
