# Micron

A developer CLI tool designed to analyze and reduce build artifact size. It identifies unnecessary files, detects optimization opportunities, and produces smaller distributable artifacts while providing detailed size reports.

## Features

- **🔍 Scan** - Analyze build artifacts and identify size issues
- **🧹 Optimize** - Safely remove unnecessary files (temp files, logs, source maps, debug artifacts)
- **📦 Pack** - Compress artifacts with zstd/gzip for efficient distribution
- **🚀 Build** - Full pipeline: scan → optimize → pack → report
- **📊 Analysis** - Detailed reports on file sizes, duplicates, and optimization opportunities

## Quick Start

### Installation

```bash
# Build from source
git clone https://github.com/micron/micron.git
cd micron
make build
```

### Basic Usage

```bash
# Scan a directory
micron scan ./dist

# Run full pipeline
micron build ./dist

# Preview optimization (dry-run)
micron optimize ./dist --dry-run

# Compress to archive
micron pack ./dist -o backup.tar.zst
```

## Commands

### `micron scan <path>` - Analyze build artifacts

```bash
# Basic scan
micron scan ./dist

# Full analysis with top 50 largest files
micron scan ./dist --analysis --top 50

# JSON output for scripting
micron scan ./dist --analysis-json --top 100

# Quick scan (no duplicates)
micron scan ./dist --quick
```

**Flags:**

- `--analysis` - Output analysis report in terminal
- `--analysis-json` - Output analysis report in JSON format
- `--json` - Output raw scan results in JSON format
- `--quick` - Quick scan without duplicate detection
- `--no-hash` - Skip file hashing for faster scanning
- `--top int` - Number of largest files to show (max 1000, default: 20)

### `micron optimize <path>` - Remove unnecessary files

```bash
# Dry-run to preview what would be deleted
micron optimize ./dist --dry-run

# Execute optimization with confirmation
micron optimize ./dist

# Auto-confirm (CI/CD usage)
micron optimize ./dist --yes

# Include docs and tests
micron optimize ./dist --remove-docs --remove-tests

# Exclude specific patterns
micron optimize ./dist --exclude "*.env" --exclude "**/secrets/*"

# Allow large file deletion
micron optimize ./dist --allow-large-files --yes
```

**Flags:**

- `--dry-run` - Preview deletions without removing files
- `--yes` - Skip confirmation prompt
- `--allow-large-files` - Allow deleting files larger than 100MB
- `--remove-tests` - Include test artifacts in optimization
- `--remove-docs` - Include documentation files in optimization
- `--remove-examples` - Include examples in optimization
- `--exclude stringArray` - Exclude files by glob (repeatable)

### `micron pack <path>` - Compress to archives

```bash
# Default zstd compression
micron pack ./dist

# Custom output path
micron pack ./dist -o ./backup.tar.zst

# Gzip format with max compression
micron pack ./dist -o ./backup.tar.gz --format gzip --level 9

# Exclude logs and temp files
micron pack ./dist --exclude "*.log" --exclude "*.tmp"
```

**Flags:**

- `--format string` - Compression format: zstd|gzip (default: "zstd")
- `--level int` - Compression level: 1-22 for zstd, 1-9 for gzip (default: 3)
- `--output string` - Output archive path (default: `<path>`.tar.zst)
- `--exclude stringArray` - Exclude files by glob (repeatable)
- `--dry-run` - Show what would be packed without creating archive

### `micron build <path>` - Full pipeline

```bash
# Full pipeline
micron build ./dist

# Scan and pack only (skip optimization)
micron build ./dist --skip-optimize

# Dry-run preview
micron build ./dist --dry-run

# Custom output with gzip
micron build ./dist -o ./release.tar.gz --format gzip --level 6 --yes
```

**Flags:**

- `--skip-optimize` - Skip optimization stage
- `--skip-pack` - Skip packing stage
- `--dry-run` - Preview changes without applying them
- `--yes` - Skip confirmation prompts
- `--format string` - Compression format: zstd|gzip (default: "zstd")
- `--level int` - Compression level (default: 3)
- `--output string` - Output archive path (default: `<path>`.tar.zst)
- `--exclude stringArray` - Exclude files by glob (repeatable)

## Configuration

Micron supports configuration files in YAML format. The default location is `~/.micron.yaml`.

### Example Configuration

```yaml
# Log level: debug, info, warn, error
log_level: info

# Verbose output
verbose: false

# Default optimization settings
optimize:
  dry_run: false
  allow_large_files: false
  remove_tests: false
  remove_docs: false
  remove_examples: false
  exclude:
    - "*.env"
    - "*.secret"

# Default pack settings
pack:
  format: zstd  # or gzip
  level: 3
  exclude:
    - "*.log"
    - "*.tmp"
```

### Configuration Examples

Copy example configurations from the `examples/` directory:

```bash
# Default configuration
cp examples/default.yaml ~/.micron.yaml

# CI/CD optimized configuration
cp examples/ci-cd.yaml ~/.micron.yaml

# Aggressive optimization
cp examples/aggressive.yaml ~/.micron.yaml
```

## Development

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Build specific platform
make build-linux
make build-windows
make build-macos
```

### Testing

```bash
# Run all tests
make test

# Run benchmarks
make bench

# Generate coverage report
make coverage
```

### Code Quality

```bash
# Format code
make fmt

# Run linter (requires golangci-lint)
make lint
```

## Architecture

The project is organized into logical packages:

```
pkg/
├── core/           # Core utilities (scanner, logger)
├── analysis/       # Analysis & reporting (analyzer, report types, reporters)
└── pipeline/       # Pipeline operations (optimizer, compressor)
```

### Core Components

- **Scanner** - Directory traversal and file analysis
- **Analyzer** - Identifies optimization opportunities
- **Optimizer** - Safely removes unnecessary files
- **Compressor** - Efficient compression with zstd/gzip

## Performance

- **Scan Performance**: Can scan 100,000 files in under 10 seconds
- **Memory Efficient**: Streaming processing for large directories
- **Concurrent**: Parallel duplicate detection and analysis

## Safety Features

- **Path Validation**: Refuses dangerous root paths (/, C:\, user home)
- **Size Limits**: Default protection against deleting large files (>100MB)
- **Hard Exclusions**: Always protects `.git`, `node_modules`, system files
- **Confirmation**: Interactive prompts for destructive operations
- **Dry Run**: Preview mode for all operations

## File Types Recognized

- **Binary** - Executables, libraries
- **Source** - Source code files
- **Resource** - Assets, media files
- **Document** - Documentation, README files
- **Config** - Configuration files
- **Temp** - Temporary files, cache
- **Debug** - Debug symbols, source maps
- **Duplicate** - Duplicate files
- **Symlink** - Symbolic links
- **Unknown** - Unrecognized file types

## Optimization Rules

Micron can safely remove:

- **Temp Files** (`.tmp`, `.temp`, cache directories)
- **Source Maps** (`.map` files)
- **Debug Artifacts** (`.pdb`, `.dbg`, debug symbols)
- **Log Files** (`.log`, logs directories)
- **Test Artifacts** (test directories, test files)
- **Documentation** (`.md`, docs directories)
- **Examples** (examples directories)
- **Build Leftovers** (`.bak`, `.old` files)

## Compression Formats

### Zstd (Default)

- Faster compression and decompression
- Better compression ratios for most files
- Modern, widely supported

### Gzip

- Maximum compatibility
- Good compression ratio
- Slower than zstd
