package report

// AnalysisReport is derived insight from a scanner.ScanResult.
// It is deterministic for a given input.
type AnalysisReport struct {
	RootPath string `json:"root_path"`

	OriginalSizeBytes int64 `json:"original_size_bytes"`
	TotalFiles        int   `json:"total_files"`
	TotalDirs         int   `json:"total_dirs"`

	TopN int `json:"top_n"`

	LargestFiles []LargestFile `json:"largest_files"`

	DuplicateSummary DuplicateSummary  `json:"duplicate_summary"`
	DuplicateGroups  []DuplicateGroup  `json:"duplicate_groups"`

	PotentialSavings PotentialSavings `json:"potential_savings"`

	FilesByType []TypeStat `json:"files_by_type"`
}

type LargestFile struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Type      string `json:"type"`
	Reason    string `json:"reason,omitempty"`
}

type DuplicateSummary struct {
	Groups      int   `json:"groups"`
	Files       int   `json:"files"`
	WastedBytes int64 `json:"wasted_bytes"`
}

type DuplicateGroup struct {
	Hash        string   `json:"hash"`
	SizeBytes   int64    `json:"size_bytes"`
	Count       int      `json:"count"`
	Files       []string `json:"files"`
	WastedBytes int64    `json:"wasted_bytes"`
}

type PotentialSavings struct {
	TotalBytes int64         `json:"total_bytes"`
	Items      []SavingsItem `json:"items"`
	Candidates []Candidate   `json:"candidates"`
}

type SavingsItem struct {
	Reason     string `json:"reason"`
	Files      int    `json:"files"`
	Bytes      int64  `json:"bytes"`
	Examples   []string `json:"examples,omitempty"`
}

type Candidate struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	Reason    string `json:"reason"`
	Type      string `json:"type"`
}

type TypeStat struct {
	Type      string `json:"type"`
	Count     int    `json:"count"`
	SizeBytes int64  `json:"size_bytes"`
}
