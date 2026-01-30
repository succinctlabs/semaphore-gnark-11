#!/usr/bin/env python3
"""
Initialize a trusted setup ceremony and generate contributor messages.

This script:
1. Downloads the Powers of Tau file (if not present)
2. Imports phase1 from PTAU
3. Initializes phase2 for the circuit
4. Uploads initial phase2 to S3
5. Generates presigned URLs for contributors
6. Creates message files for each contributor

Requires:
- AWS credentials configured (for S3 upload and presigned URL generation)
- The semaphore-gnark-11 binary built
"""

import argparse
import sys
from pathlib import Path

from trusted_setup import (
    REPO_ROOT,
    build_binary,
    download_ptau,
    generate_messages,
    generate_presigned_urls,
    get_ptau_filename,
    phase1_import,
    phase2_init,
    phase2_upload,
)

# Configuration
NB_CONSTRAINTS_LOG2 = 24

# Paths
TRUSTED_SETUP_DIR = REPO_ROOT / "trusted-setup"
MESSAGES_DIR = TRUSTED_SETUP_DIR / "messages"

PTAU_PATH = TRUSTED_SETUP_DIR / get_ptau_filename(NB_CONSTRAINTS_LOG2)
PHASE1_PATH = TRUSTED_SETUP_DIR / "phase1"
PHASE2_PATH = TRUSTED_SETUP_DIR / "phase2"
EVALS_PATH = TRUSTED_SETUP_DIR / "evals"


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Initialize a trusted setup ceremony",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Example:
    python scripts/initialize_trusted_setup.py \\
        --bucket-name my-trusted-setup-bucket \\
        --circuit-path /path/to/circuit.bin \\
        --contribution-count 5 \\
        --phase1-beacon-round 1234567 \\
        --phase2-beacon-round 1235567
        """,
    )
    parser.add_argument(
        "--bucket-name",
        required=True,
        help="S3 bucket name for storing ceremony files",
    )
    parser.add_argument(
        "--circuit-path",
        required=True,
        type=Path,
        help="Path to the circuit R1CS file",
    )
    parser.add_argument(
        "--contribution-count",
        required=True,
        type=int,
        help="Number of expected contributions",
    )
    parser.add_argument(
        "--phase1-beacon-round",
        required=True,
        type=int,
        help="Drand round number for the Phase1 beacon",
    )
    parser.add_argument(
        "--phase2-beacon-round",
        required=True,
        type=int,
        help="Drand round number for the Phase2 beacon",
    )

    args = parser.parse_args()

    if not args.circuit_path.exists():
        print(f"Error: Circuit file not found: {args.circuit_path}")
        return 1

    if args.contribution_count < 1:
        print("Error: Contribution count must be at least 1")
        return 1

    if args.phase1_beacon_round <= 0 or args.phase2_beacon_round <= 0:
        print("Error: Beacon rounds must be positive drand round numbers")
        return 1

    print("=" * 60)
    print("Trusted Setup Initialization")
    print("=" * 60)
    print(f"Bucket: {args.bucket_name}")
    print(f"Circuit: {args.circuit_path}")
    print(f"Contributors: {args.contribution_count}")
    print(f"Phase1 beacon round: {args.phase1_beacon_round}")
    print(f"Phase2 beacon round: {args.phase2_beacon_round}")
    print("=" * 60)

    # Create directories
    TRUSTED_SETUP_DIR.mkdir(parents=True, exist_ok=True)
    MESSAGES_DIR.mkdir(parents=True, exist_ok=True)

    try:
        build_binary()
        download_ptau(PTAU_PATH, NB_CONSTRAINTS_LOG2)
        phase1_import(PTAU_PATH, PHASE1_PATH)
        phase2_init(
            PHASE1_PATH,
            args.circuit_path,
            PHASE2_PATH,
            EVALS_PATH,
            beacon_round=args.phase1_beacon_round,
        )
        phase2_upload(args.bucket_name)

        # Generate presigned URLs for each contributor
        # URL index 0 → first contributor uploads phase2-0
        # URL index 1 → second contributor uploads phase2-1, etc.
        urls = generate_presigned_urls(args.bucket_name, args.contribution_count)
        generate_messages(args.bucket_name, urls, MESSAGES_DIR)

        print()
        print("=" * 60)
        print("Initialization complete!")
        print("=" * 60)
        print(f"Messages generated in: {MESSAGES_DIR}")
        print()
        print("Next steps:")
        print("1. Send each contributor their message file (msg1.txt, msg2.txt, ...)")
        print("2. Wait for each contribution to complete in order")
        print(f"3. Verify each contribution with: ./semaphore-gnark-11 p2v <index> {args.bucket_name}")
        print("4. After all contributions, extract keys with: ./semaphore-gnark-11 key ...")
        return 0

    except Exception as e:
        print(f"\nError: {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
