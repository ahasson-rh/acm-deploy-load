# Phase 1 Manual Testing Guide

## 1. Build the Binary
```bash
cd scripts/workload-image-curator
make build
```

## 2. Basic Help & Version
```bash
# Show all available flags
./workload-image-curator --help

# Show verbose flag details
./workload-image-curator -v --help
```

## 3. Test Random Selection Mode (Default)
```bash
# Random selection with default 50 images
./workload-image-curator -c 50 --stdout --no-files

# Custom random count
./workload-image-curator -c 100 --stdout --no-files

# Random with file output
./workload-image-curator -c 20 --output-prefix "test-random"

# Verify files created
ls -lh test-random_*.{json,txt}
cat test-random_*.txt
```

## 4. Test Size-Based Strategy Mode
```bash
# Size-based distribution
./workload-image-curator --strategy "small:10,medium:30,large:10" --stdout --no-files

# Different distribution
./workload-image-curator --strategy "small:20,medium:20,large:10" --stdout --no-files

# With custom thresholds
./workload-image-curator \
  --strategy "small:10,medium:30,large:10" \
  --size-small-threshold 31457280 \
  --size-large-threshold 157286400 \
  --stdout --no-files
```

## 5. Test Output Combinations
```bash
# Stdout only
./workload-image-curator -c 5 --stdout --no-files

# Files only (no stdout)
./workload-image-curator -c 5 --no-files

# Both stdout and files
./workload-image-curator -c 5 --stdout

# Custom file paths
./workload-image-curator \
  -c 5 \
  --output-json /tmp/custom-images.json \
  --output-txt /tmp/custom-images.txt \
  --stdout

# Verify JSON schema
cat /tmp/custom-images.json | jq '.[0]'
```

## 6. Test Dry-Run Mode
```bash
# Dry-run with stdout (no network/validation calls)
./workload-image-curator \
  --strategy "small:10,medium:30,large:10" \
  --target-registry bastion:5000 \
  --target-org operator-containers \
  --dry-run \
  --stdout --no-files
```

## 7. Test Logging & Progress
```bash
# Verbose logging
./workload-image-curator -c 10 --verbose --stdout --no-files

# Quiet mode (suppress progress)
./workload-image-curator -c 10 --quiet --stdout --no-files
```

## 8. Test Concurrency Flags
```bash
# Custom worker counts
./workload-image-curator \
  -c 20 \
  --workers 10 \
  --inspect-workers 20 \
  --rate-limit 15.0 \
  --stdout --no-files
```

## 9. Test Validation Flags
```bash
# Skip validation
./workload-image-curator \
  -c 10 \
  --skip-validation \
  --stdout --no-files

# Custom validation timeout
./workload-image-curator \
  -c 10 \
  --validation-timeout 5 \
  --stdout --no-files
```

## 10. Test Registry & Mirroring Flags
```bash
# With registry (dry-run, no actual mirroring)
./workload-image-curator \
  --strategy "small:5,medium:5,large:5" \
  --target-registry bastion:5000 \
  --target-org operator-containers \
  --pull-secret /path/to/secret \
  --dry-run \
  --stdout --no-files

# With ignore-existing flag
./workload-image-curator \
  --strategy "small:5,medium:5,large:5" \
  --target-registry bastion:5000 \
  --ignore-existing \
  --dry-run \
  --stdout --no-files

# Auto-assume yes
./workload-image-curator \
  --strategy "small:5,medium:5,large:5" \
  --target-registry bastion:5000 \
  --assume-yes \
  --dry-run \
  --stdout --no-files
```

## 11. Test Environment Variables (Future)
```bash
# Set Pyxis API endpoint
export PYXIS_BASE_URL="https://custom.api.com"
./workload-image-curator -c 5 --stdout --no-files
unset PYXIS_BASE_URL
```

## 12. Verify Output Format Matches Python
```bash
# Check JSON structure
./workload-image-curator -c 5 --output-prefix phase1-test
cat phase1-test_*.json | jq 'length'  # Should show 5
cat phase1-test_*.json | jq '.[0] | keys'  # Should show: ["operator", "csv_name", "sha_digest", "quay_image"]

# Check TXT format (one per line)
wc -l phase1-test_*.txt  # Should match image count
```

## 13. Error Cases
```bash
# Invalid strategy format (will use random fallback)
./workload-image-curator --strategy "invalid" --stdout --no-files

# Zero count
./workload-image-curator -c 0 --stdout --no-files

# Negative count
./workload-image-curator -c -5 --stdout --no-files
```

## 14. Integration Test (Full Workflow)
```bash
# Simulate full workflow without actual mirroring
./workload-image-curator \
  --strategy "small:10,medium:20,large:5" \
  --size-small-threshold 52428800 \
  --size-large-threshold 209715200 \
  --target-registry bastion:5000 \
  --target-org operator-containers \
  --workers 5 \
  --inspect-workers 10 \
  --rate-limit 10.0 \
  --validation-timeout 8 \
  --skip-validation \
  --dry-run \
  --assume-yes \
  --stdout \
  --output-prefix phase1-final-test \
  --verbose

# Verify output files
ls -lh phase1-final-test_*
jq 'length' phase1-final-test_*.json
head phase1-final-test_*.txt
```

## Notes

**Phase 1 Limitations:**
- Pyxis API calls will attempt to connect (will timeout if no network)
- Image validation will attempt (will be slow without skopeo)
- Actual mirroring is not implemented (use --dry-run or --skip-validation)

**Expected Behavior:**
- `--stdout --no-files` + `--skip-validation` + `--dry-run` = fast CLI testing
- Binary accepts all flags and parses them correctly
- Help output shows all available options
- File creation works (JSON/TXT formats)
- Configuration defaults are sensible

