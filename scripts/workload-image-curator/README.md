# Workload Image Curator

Fetch and mirror operator container images for ACS (Advanced Cluster Security) testing workloads.

## Build

```bash
make build
```

## Install

```bash
make install
```

## Usage

### Random Selection
```bash
workload-image-curator -c 50 --stdout
```

### Size-Based Selection
```bash
workload-image-curator --strategy "small:10,medium:30,large:10" --stdout
```

### With Mirroring
```bash
workload-image-curator \
  --strategy "small:10,medium:30,large:10" \
  --target-registry bastion:5000 \
  --target-org operator-containers \
  --stdout
```

### Dry Run
```bash
workload-image-curator \
  --strategy "small:10,medium:30,large:10" \
  --target-registry bastion:5000 \
  --dry-run \
  --stdout
```

## Features

- **Two selection modes**: Random (`-c COUNT`) or size-based (`--strategy`)
- **Size categorization**: Small (<50MB), Medium (50-200MB), Large (>200MB)
- **Registry pre-assessment**: Skip already-mirrored images
- **Concurrent operations**: Configurable worker pools for Pyxis queries, validation, and mirroring
- **Dry run mode**: Preview without validation or mirroring
- **Multiple output formats**: stdout, JSON, TXT files
- **Backward compatible**: Output format matches Python implementation

## Testing

```bash
make test
```

## Architecture

Phase 1 implements:
- Pyxis API client with retry logic
- Image validation (skopeo + HTTP fallback)
- Output formatting (JSON + TXT)
- Configuration management
- CLI interface

Future phases will add:
- Concurrent worker pools
- Size-based categorization
- Registry pre-assessment
- Actual image mirroring
