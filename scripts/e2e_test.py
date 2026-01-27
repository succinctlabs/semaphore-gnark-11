#!/usr/bin/env python3
"""
E2E test for the trusted setup process using a small MiMC circuit.
No S3 required - runs entirely locally.
"""

import os
import subprocess
import sys
import urllib.request
from pathlib import Path

# Configuration
NB_CONSTRAINTS_LOG2 = 9
PTAU_URL = f"https://storage.googleapis.com/zkevm/ptau/powersOfTau28_hez_final_{NB_CONSTRAINTS_LOG2:02d}.ptau"
PTAU_FILENAME = f"powersOfTau28_hez_final_{NB_CONSTRAINTS_LOG2:02d}.ptau"

# Paths
REPO_ROOT = Path(__file__).parent.parent
BUILD_DIR = REPO_ROOT / "build"
BINARY = REPO_ROOT / "semaphore-gnark-11"

PTAU_PATH = BUILD_DIR / PTAU_FILENAME
PHASE1_PATH = BUILD_DIR / "phase1"
PHASE2_PATH = BUILD_DIR / "phase2"
EVALS_PATH = BUILD_DIR / "evals"
R1CS_PATH = BUILD_DIR / "r1cs"
PK_PATH = BUILD_DIR / "pk"
VK_PATH = BUILD_DIR / "vk"


def run_cmd(cmd: list[str], cwd: Path | None = None) -> None:
    """Run a command and check for errors."""
    print(f"Running: {' '.join(cmd)}")
    result = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
    if result.returncode != 0:
        print(f"STDOUT: {result.stdout}")
        print(f"STDERR: {result.stderr}")
        raise RuntimeError(f"Command failed with exit code {result.returncode}")
    if result.stdout:
        print(result.stdout)


def download_ptau() -> None:
    """Download the Powers of Tau file if not present."""
    if PTAU_PATH.exists():
        print(f"PTAU file already exists: {PTAU_PATH}")
        return

    print(f"Downloading PTAU from {PTAU_URL}...")
    urllib.request.urlretrieve(PTAU_URL, PTAU_PATH)
    print(f"Downloaded to {PTAU_PATH}")


def build_binary() -> None:
    """Build the Go binary if not present or outdated."""
    print("Building Go binary...")
    run_cmd(["go", "build"], cwd=REPO_ROOT)
    if not BINARY.exists():
        raise RuntimeError(f"Binary not found after build: {BINARY}")


def generate_r1cs() -> None:
    """Generate the R1CS file by running the Go test."""
    if R1CS_PATH.exists():
        print(f"R1CS file already exists: {R1CS_PATH}")
        return

    print("Generating R1CS from MiMC circuit...")
    run_cmd(["go", "test", "-v", "-run", "TestGenerateR1CS", "./test/"], cwd=REPO_ROOT)


def run_phase1_import() -> None:
    """Import phase1 from PTAU file."""
    print("Importing phase1 from PTAU...")
    run_cmd([str(BINARY), "p1i", str(PTAU_PATH), str(PHASE1_PATH)])


def run_phase2_init() -> None:
    """Initialize phase2."""
    print("Initializing phase2...")
    run_cmd([
        str(BINARY), "p2n",
        str(PHASE1_PATH),
        str(R1CS_PATH),
        str(PHASE2_PATH),
        str(EVALS_PATH),
    ])


def run_e2e_contributions() -> None:
    """Run the Go e2e test which does local contributions."""
    print("Running Go e2e test (local contributions + key extraction + proof)...")
    run_cmd(["go", "test", "-v", "-run", "TestEndToEnd", "./test/"], cwd=REPO_ROOT)


def main() -> int:
    print("=" * 60)
    print("E2E Trusted Setup Test")
    print("=" * 60)

    # Create build directory
    BUILD_DIR.mkdir(parents=True, exist_ok=True)
    (BUILD_DIR / "contributions").mkdir(parents=True, exist_ok=True)

    try:
        build_binary()
        download_ptau()
        generate_r1cs()
        run_e2e_contributions()

        print()
        print("=" * 60)
        print("E2E test PASSED!")
        print("=" * 60)
        print(f"Artifacts in {BUILD_DIR}:")
        for f in BUILD_DIR.iterdir():
            if f.is_file():
                print(f"  - {f.name}")
        return 0

    except Exception as e:
        print(f"\nE2E test FAILED: {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
