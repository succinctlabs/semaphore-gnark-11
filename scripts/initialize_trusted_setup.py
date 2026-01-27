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
import os
import subprocess
import sys
import urllib.request
from pathlib import Path

# Configuration
NB_CONSTRAINTS_LOG2 = 24
PTAU_URL = f"https://storage.googleapis.com/zkevm/ptau/powersOfTau28_hez_final_{NB_CONSTRAINTS_LOG2}.ptau"
PTAU_FILENAME = f"powersOfTau28_hez_final_{NB_CONSTRAINTS_LOG2}.ptau"

# Paths
REPO_ROOT = Path(__file__).parent.parent
TRUSTED_SETUP_DIR = REPO_ROOT / "trusted-setup"
MESSAGES_DIR = TRUSTED_SETUP_DIR / "messages"
BINARY = REPO_ROOT / "semaphore-gnark-11"

PTAU_PATH = TRUSTED_SETUP_DIR / PTAU_FILENAME
PHASE1_PATH = TRUSTED_SETUP_DIR / "phase1"
PHASE2_PATH = TRUSTED_SETUP_DIR / "phase2"
EVALS_PATH = TRUSTED_SETUP_DIR / "evals"

MESSAGE_TEMPLATE = """Hey, you have been chosen to perform contribution #{index} to the trusted setup!

```bash
git clone https://github.com/succinctlabs/semaphore-gnark-11.git
cd semaphore-gnark-11
go build
mkdir trusted-setup

./semaphore-gnark-11 p2c "{url}" {bucket_name}
```

Don't hesitate if you have any questions.
"""


def run_cmd(cmd: list[str], capture_output: bool = False) -> subprocess.CompletedProcess:
    """Run a command and check for errors."""
    print(f"Running: {' '.join(cmd)}")
    result = subprocess.run(cmd, capture_output=capture_output, text=True)
    if result.returncode != 0:
        if capture_output:
            print(f"STDOUT: {result.stdout}")
            print(f"STDERR: {result.stderr}")
        raise RuntimeError(f"Command failed with exit code {result.returncode}")
    return result


def download_ptau() -> None:
    """Download the Powers of Tau file if not present."""
    if PTAU_PATH.exists():
        print(f"PTAU file already exists: {PTAU_PATH}")
        return

    print(f"Downloading PTAU from {PTAU_URL}...")
    print("(This is a large file, may take a while...)")
    urllib.request.urlretrieve(PTAU_URL, PTAU_PATH)
    print(f"Downloaded to {PTAU_PATH}")


def build_binary() -> None:
    """Build the Go binary if not present."""
    if BINARY.exists():
        print(f"Binary already exists: {BINARY}")
        return

    print("Building Go binary...")
    run_cmd(["go", "build"], cwd=REPO_ROOT)
    if not BINARY.exists():
        raise RuntimeError(f"Binary not found after build: {BINARY}")


def run_phase1_import() -> None:
    """Import phase1 from PTAU file."""
    if PHASE1_PATH.exists():
        print(f"Phase1 already exists: {PHASE1_PATH}")
        return

    print("Importing phase1 from PTAU...")
    run_cmd([str(BINARY), "p1i", str(PTAU_PATH), str(PHASE1_PATH)])


def run_phase2_init(circuit_path: Path) -> None:
    """Initialize phase2."""
    if PHASE2_PATH.exists():
        print(f"Phase2 already exists: {PHASE2_PATH}")
        return

    print("Initializing phase2...")
    run_cmd([
        str(BINARY), "p2n",
        str(PHASE1_PATH),
        str(circuit_path),
        str(PHASE2_PATH),
        str(EVALS_PATH),
    ])


def upload_phase2(bucket_name: str) -> None:
    """Upload initial phase2 to S3."""
    print(f"Uploading phase2 to S3 bucket: {bucket_name}...")
    run_cmd([str(BINARY), "p2u", bucket_name])


def generate_presigned_urls(bucket_name: str, count: int) -> list[tuple[int, str]]:
    """Generate presigned URLs and parse the output."""
    print(f"Generating {count} presigned URLs...")
    result = run_cmd([str(BINARY), "presigned", bucket_name, str(count)], capture_output=True)

    urls = []
    for line in result.stdout.strip().split("\n"):
        if not line:
            continue
        # Format: "0: https://..."
        index_str, url = line.split(": ", 1)
        urls.append((int(index_str), url))

    return urls


def generate_messages(bucket_name: str, urls: list[tuple[int, str]]) -> None:
    """Generate contributor message files."""
    MESSAGES_DIR.mkdir(parents=True, exist_ok=True)

    # Skip index 0 (that's the initial phase2 uploaded by coordinator)
    # Contributors get URLs for phase2-1, phase2-2, etc.
    for index, url in urls:
        if index == 0:
            continue  # Index 0 is for initial upload, not a contributor

        msg_path = MESSAGES_DIR / f"msg{index}.txt"
        msg_content = MESSAGE_TEMPLATE.format(
            index=index,
            url=url,
            bucket_name=bucket_name,
        )
        msg_path.write_text(msg_content)
        print(f"Created: {msg_path}")


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Initialize a trusted setup ceremony",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Example:
    python scripts/initialize_trusted_setup.py \\
        --bucket-name my-trusted-setup-bucket \\
        --circuit-path /path/to/circuit.bin \\
        --contribution-count 5
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

    args = parser.parse_args()

    if not args.circuit_path.exists():
        print(f"Error: Circuit file not found: {args.circuit_path}")
        return 1

    if args.contribution_count < 1:
        print("Error: Contribution count must be at least 1")
        return 1

    print("=" * 60)
    print("Trusted Setup Initialization")
    print("=" * 60)
    print(f"Bucket: {args.bucket_name}")
    print(f"Circuit: {args.circuit_path}")
    print(f"Contributors: {args.contribution_count}")
    print("=" * 60)

    # Create directories
    TRUSTED_SETUP_DIR.mkdir(parents=True, exist_ok=True)
    MESSAGES_DIR.mkdir(parents=True, exist_ok=True)

    try:
        build_binary()
        download_ptau()
        run_phase1_import()
        run_phase2_init(args.circuit_path)
        upload_phase2(args.bucket_name)

        # Generate presigned URLs (count + 1 because index 0 is initial upload)
        urls = generate_presigned_urls(args.bucket_name, args.contribution_count + 1)
        generate_messages(args.bucket_name, urls)

        print()
        print("=" * 60)
        print("Initialization complete!")
        print("=" * 60)
        print(f"Messages generated in: {MESSAGES_DIR}")
        print()
        print("Next steps:")
        print("1. Send each contributor their message file (msg1.txt, msg2.txt, ...)")
        print("2. Wait for each contribution to complete in order")
        print("3. Verify each contribution with: ./semaphore-gnark-11 p2v <index> <bucket>")
        print("4. After all contributions, extract keys with: ./semaphore-gnark-11 key ...")
        return 0

    except Exception as e:
        print(f"\nError: {e}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
