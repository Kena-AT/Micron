package analyzer

import (
	"testing"

	"github.com/micron/micron/pkg/scanner"
)

func TestAnalyzeDeterminismLargestFiles(t *testing.T) {
	scan := &scanner.ScanResult{
		RootPath:   "/root",
		TotalFiles: 3,
		TotalDirs:  1,
		TotalSize:  300,
		FilesByType: map[scanner.FileType]int{
			scanner.TypeUnknown: 3,
		},
		SizeByType: map[scanner.FileType]int64{
			scanner.TypeUnknown: 300,
		},
		Files: []scanner.FileEntry{
			{Path: "/root/b", Name: "b", Size: 100, Type: scanner.TypeUnknown},
			{Path: "/root/a", Name: "a", Size: 100, Type: scanner.TypeUnknown},
			{Path: "/root/c", Name: "c", Size: 200, Type: scanner.TypeUnknown},
		},
	}

	rep := Analyze(scan, 20)
	if len(rep.LargestFiles) != 3 {
		t.Fatalf("expected 3 largest files, got %d", len(rep.LargestFiles))
	}

	// Size desc => c first. For same size (a,b) => path asc => a then b
	if rep.LargestFiles[0].Path != "/root/c" {
		t.Fatalf("expected first /root/c, got %s", rep.LargestFiles[0].Path)
	}
	if rep.LargestFiles[1].Path != "/root/a" {
		t.Fatalf("expected second /root/a, got %s", rep.LargestFiles[1].Path)
	}
	if rep.LargestFiles[2].Path != "/root/b" {
		t.Fatalf("expected third /root/b, got %s", rep.LargestFiles[2].Path)
	}
}

func TestAnalyzeDuplicateGroupOrdering(t *testing.T) {
	scan := &scanner.ScanResult{
		RootPath: "/root",
		FilesByType: map[scanner.FileType]int{},
		SizeByType:  map[scanner.FileType]int64{},
		Duplicates: []scanner.DuplicateGroup{
			{Hash: "b", Size: 10, Count: 2, Files: []string{"/root/z", "/root/a"}, WastedSpace: 10},
			{Hash: "a", Size: 10, Count: 3, Files: []string{"/root/m", "/root/n", "/root/o"}, WastedSpace: 20},
			{Hash: "c", Size: 10, Count: 2, Files: []string{"/root/x", "/root/y"}, WastedSpace: 20},
		},
	}

	rep := Analyze(scan, 20)
	if len(rep.DuplicateGroups) != 3 {
		t.Fatalf("expected 3 duplicate groups, got %d", len(rep.DuplicateGroups))
	}

	// wasted desc: 20,20,10. tie -> hash asc => a then c
	if rep.DuplicateGroups[0].Hash != "a" {
		t.Fatalf("expected first hash a, got %s", rep.DuplicateGroups[0].Hash)
	}
	if rep.DuplicateGroups[1].Hash != "c" {
		t.Fatalf("expected second hash c, got %s", rep.DuplicateGroups[1].Hash)
	}
	if rep.DuplicateGroups[2].Hash != "b" {
		t.Fatalf("expected third hash b, got %s", rep.DuplicateGroups[2].Hash)
	}

	// files inside group sorted asc
	if rep.DuplicateGroups[2].Files[0] != "/root/a" {
		t.Fatalf("expected group files sorted asc")
	}
}

func TestPotentialSavingsReasons(t *testing.T) {
	scan := &scanner.ScanResult{
		RootPath: "/root",
		FilesByType: map[scanner.FileType]int{},
		SizeByType:  map[scanner.FileType]int64{},
		Files: []scanner.FileEntry{
			{Path: "/root/a.tmp", Name: "a.tmp", Extension: ".tmp", Size: 10, Type: scanner.TypeTemp},
			{Path: "/root/app.js.map", Name: "app.js.map", Extension: ".map", Size: 20, Type: scanner.TypeUnknown},
			{Path: "/root/app.pdb", Name: "app.pdb", Extension: ".pdb", Size: 30, Type: scanner.TypeDebug},
			{Path: "/root/logs/app.log", Name: "app.log", Extension: ".log", Size: 40, Type: scanner.TypeUnknown},
			{Path: "/root/test/test.bin", Name: "test.bin", Extension: ".bin", Size: 50, Type: scanner.TypeBinary},
			{Path: "/root/README.md", Name: "README.md", Extension: ".md", Size: 60, Type: scanner.TypeDocument},
			{Path: "/root/examples/x.txt", Name: "x.txt", Extension: ".txt", Size: 70, Type: scanner.TypeDocument},
			{Path: "/root/x.bak", Name: "x.bak", Extension: ".bak", Size: 80, Type: scanner.TypeUnknown},
		},
		Duplicates: []scanner.DuplicateGroup{
			{Hash: "dup", Size: 100, Count: 2, Files: []string{"/root/d1", "/root/d2"}, WastedSpace: 100},
		},
	}

	rep := Analyze(scan, 20)
	if rep.PotentialSavings.TotalBytes <= 0 {
		t.Fatalf("expected positive potential savings")
	}

	// should include duplicate as item
	foundDup := false
	for _, it := range rep.PotentialSavings.Items {
		if it.Reason == ReasonDuplicate {
			foundDup = true
			if it.Bytes != 100 {
				t.Fatalf("expected duplicate bytes 100, got %d", it.Bytes)
			}
		}
	}
	if !foundDup {
		t.Fatalf("expected duplicate savings item")
	}
}
