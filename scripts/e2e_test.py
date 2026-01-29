#!/usr/bin/env python3
"""
E2E test for the trusted setup process using a small MiMC circuit and Minio.

This test:
1. Starts Minio (local S3)
2. Runs the full trusted setup initialization
3. Simulates multiple contributor contributions
4. Verifies all contributions
5. Extracts keys
6. Runs proof generation/verification

Requires:
- Docker and docker-compose
"""

import shutil
import subprocess
import sys
import time
from pathlib import Path

from dotenv import dotenv_values

from trusted_setup import (
    REPO_ROOT,
    build_binary,
    download_ptau,
    extract_keys,
    generate_presigned_urls,
    generate_r1cs,
    get_ptau_filename,
    phase1_import,
    phase2_contribute,
    phase2_init,
    phase2_upload,
    phase2_verify,
    run_cmd,
)

# Configuration
NB_CONSTRAINTS_LOG2 = 9  # Small for testing
NUM_CONTRIBUTIONS = 3

# Paths
SCRIPTS_DIR = Path(__file__).parent
BUILD_DIR = REPO_ROOT / "build"
TRUSTED_SETUP_DIR = REPO_ROOT / "trusted-setup"

PTAU_PATH = BUILD_DIR / get_ptau_filename(NB_CONSTRAINTS_LOG2)
R1CS_PATH = BUILD_DIR / "r1cs"

# Phase files must be in trusted-setup/ because p2u hardcodes that path
PHASE1_PATH = TRUSTED_SETUP_DIR / "phase1"
PHASE2_PATH = TRUSTED_SETUP_DIR / "phase2"
EVALS_PATH = TRUSTED_SETUP_DIR / "evals"

# Drand rounds for reproducible E2E
PHASE1_BEACON_ROUND = 1000
PHASE2_BEACON_ROUND = 2000


MINIO_FILE_ENV = dotenv_values(SCRIPTS_DIR / "minio.env")
BUCKET_NAME = MINIO_FILE_ENV.get("BUCKET_NAME", "e2e-trusted-setup")

# Minio credentials (from minio.env)
MINIO_ENV = {
    "AWS_ACCESS_KEY_ID": MINIO_FILE_ENV.get("ACCESS_KEY", ""),
    "AWS_SECRET_ACCESS_KEY": MINIO_FILE_ENV.get("SECRET_KEY", ""),
    "CUSTOM_ENDPOINT": "http://127.0.0.1:9000/",
}


def start_minio() -> None:
    """Start Minio using docker-compose."""
    print("Starting Minio...")
    subprocess.run(
        ["docker", "compose", "up", "-d"],
        cwd=SCRIPTS_DIR,
        check=True,
    )
    # Wait for Minio to be ready with health check
    print("Waiting for Minio to be ready...")
    import urllib.request
    import urllib.error
    for i in range(30):
        try:
            urllib.request.urlopen("http://127.0.0.1:9000/minio/health/live", timeout=1)
            print("Minio is ready!")
            return
        except Exception:
            time.sleep(1)
    raise RuntimeError("Minio failed to start within 30 seconds")


def stop_minio() -> None:
    """Stop Minio."""
    print("Stopping Minio...")
    subprocess.run(
        ["docker", "compose", "down"],
        cwd=SCRIPTS_DIR,
        check=False,
    )


def run_proof_test() -> None:
    """Run the Go test to verify keys work for proving."""
    print("Running proof verification test...")
    run_cmd(["go", "test", "-v", "-run", "TestProveAndVerifyV2", "./test/"], cwd=REPO_ROOT)


def main() -> int:
    print("=" * 60)
    print("E2E Trusted Setup Test (with Minio)")
    print("=" * 60)

    if not MINIO_ENV["AWS_ACCESS_KEY_ID"] or not MINIO_ENV["AWS_SECRET_ACCESS_KEY"]:
        raise RuntimeError("minio.env missing ACCESS_KEY or SECRET_KEY")

    # Clear and create directories
    if BUILD_DIR.exists():
        shutil.rmtree(BUILD_DIR)
    if TRUSTED_SETUP_DIR.exists():
        shutil.rmtree(TRUSTED_SETUP_DIR)
    BUILD_DIR.mkdir(parents=True, exist_ok=True)
    TRUSTED_SETUP_DIR.mkdir(parents=True, exist_ok=True)

    minio_started = False

    try:
        # Build and prepare
        build_binary(force=True)
        download_ptau(PTAU_PATH, NB_CONSTRAINTS_LOG2)
        generate_r1cs(R1CS_PATH)

        # Start Minio
        start_minio()
        minio_started = True

        # Phase 1: Import
        phase1_import(PTAU_PATH, PHASE1_PATH, env=MINIO_ENV)

        # Phase 2: Initialize
        phase2_init(
            PHASE1_PATH,
            R1CS_PATH,
            PHASE2_PATH,
            EVALS_PATH,
            beacon_round=PHASE1_BEACON_ROUND,
            env=MINIO_ENV,
        )

        # Upload initial phase2 to Minio
        phase2_upload(BUCKET_NAME, env=MINIO_ENV)

        # Generate presigned URLs for contributions
        # Need exactly NUM_CONTRIBUTIONS URLs (0 through NUM_CONTRIBUTIONS-1)
        urls = generate_presigned_urls(BUCKET_NAME, NUM_CONTRIBUTIONS, env=MINIO_ENV)

        # Make contributions
        # URL index 0 is for first contributor (uploads phase2-0, downloads phase2)
        # URL index 1 is for second contributor (uploads phase2-1, downloads phase2-0)
        contribution_hashes = []
        for index, url in urls:
            print()
            print(f"--- Contribution {index + 1} (phase2-{index}) ---")
            hash_val = phase2_contribute(url, BUCKET_NAME, env=MINIO_ENV)
            contribution_hashes.append((index, hash_val))
            print(f"Contribution hash: {hash_val}")

        # Verify all contributions
        print()
        print("--- Verifying contributions ---")
        for index, _ in contribution_hashes:
            phase2_verify(index, BUCKET_NAME, env=MINIO_ENV)
            print(f"Contribution {index} verified!")

        # Download final contribution and extract keys
        print()
        print("--- Extracting keys ---")
        final_index = NUM_CONTRIBUTIONS - 1
        final_phase2_path = TRUSTED_SETUP_DIR / f"phase2-{final_index}"

        # Download the final phase2 file
        run_cmd([
            "curl", "-o", str(final_phase2_path),
            f"http://127.0.0.1:9000/{BUCKET_NAME}/phase2-{final_index}",
        ])

        extract_keys(
            PHASE1_PATH,
            final_phase2_path,
            EVALS_PATH,
            R1CS_PATH,
            phase1_beacon_round=PHASE1_BEACON_ROUND,
            phase2_beacon_round=PHASE2_BEACON_ROUND,
            env=MINIO_ENV,
        )

        # Move generated keys to build dir (for TestProveAndVerifyV2)
        if (REPO_ROOT / "pk").exists():
            shutil.copy(REPO_ROOT / "pk", BUILD_DIR / "pk")
        if (REPO_ROOT / "vk").exists():
            shutil.copy(REPO_ROOT / "vk", BUILD_DIR / "vk")

        # Verify keys work
        print()
        print("--- Testing proof generation ---")
        run_proof_test()

        print()
        print("=" * 60)
        print("E2E test PASSED!")
        print("=" * 60)
        print(f"Artifacts in {BUILD_DIR}:")
        for f in sorted(BUILD_DIR.iterdir()):
            if f.is_file():
                print(f"  - {f.name}")

        print()
        print("Contributions:")
        for index, hash_val in contribution_hashes:
            print(f"  - phase2-{index}: {hash_val}")

        return 0

    except Exception as e:
        print(f"\nE2E test FAILED: {e}")
        import traceback
        traceback.print_exc()
        return 1

    finally:
        if minio_started:
            stop_minio()


if __name__ == "__main__":
    sys.exit(main())
