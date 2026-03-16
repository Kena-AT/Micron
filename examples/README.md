# Micron Configuration Examples

This directory contains example configuration files for the Micron CLI.

## Usage

Copy an example config to your home directory:

```bash
# Linux/macOS
cp default.yaml ~/.micron.yaml

# Windows
copy default.yaml %USERPROFILE%\.micron.yaml
```

## Available Examples

- **default.yaml** - Standard configuration for most projects
- **ci-cd.yaml** - Optimized for CI/CD pipelines (non-interactive)
- **aggressive.yaml** - Aggressive optimization (removes more file types)

## Configuration Priority

1. Command-line flags (highest priority)
2. Environment variables
3. Config file (`~/.micron.yaml` or specified via `--config`)
4. Default values (lowest priority)
