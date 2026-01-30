#!/bin/bash
set -e

# Runs all Go tests in the correct order with proper artifact setup.
# Usage: ./scripts/run_go_tests.sh

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$REPO_ROOT/build"
PTAU_URL="https://storage.googleapis.com/zkevm/ptau/powersOfTau28_hez_final_09.ptau"
PTAU_FILE="$BUILD_DIR/powersOfTau28_hez_final_09.ptau"

cd "$REPO_ROOT"

echo "=== Cleaning artifacts ==="
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR/contributions"

echo "=== Downloading PTAU ==="
if [ ! -f "$PTAU_FILE" ]; then
    curl -o "$PTAU_FILE" "$PTAU_URL"
else
    echo "Already exists: $PTAU_FILE"
fi

echo "=== TestGenerateR1CS ==="
go test -v -run TestGenerateR1CS ./test/

echo "=== TestEndToEnd ==="
go test -v -run TestEndToEnd ./test/

echo "=== TestProveAndVerifyV2 ==="
go test -v -run TestProveAndVerifyV2 ./test/

echo "=== All tests passed ==="
