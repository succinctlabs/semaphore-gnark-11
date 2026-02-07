#!/usr/bin/env python3
"""
Shared logic for trusted setup scripts.
"""

import os
import subprocess
import urllib.request
from pathlib import Path

# Paths
REPO_ROOT = Path(__file__).parent.parent
BINARY = REPO_ROOT / "semaphore-gnark-11"

# PTAU URLs
PTAU_BASE_URL = "https://storage.googleapis.com/zkevm/ptau"

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


def run_cmd(
    cmd: list[str],
    cwd: Path | None = None,
    capture_output: bool = False,
    env: dict | None = None,
) -> subprocess.CompletedProcess:
    """Run a command and check for errors."""
    print(f"Running: {' '.join(cmd)}")

    # Merge with current environment if env provided
    run_env = None
    if env:
        run_env = os.environ.copy()
        run_env.update(env)

    result = subprocess.run(cmd, cwd=cwd, capture_output=capture_output, text=True, env=run_env)
    if result.returncode != 0:
        if capture_output:
            print(f"STDOUT: {result.stdout}")
            print(f"STDERR: {result.stderr}")
        raise RuntimeError(f"Command failed with exit code {result.returncode}")
    if not capture_output and result.stdout:
        print(result.stdout)
    return result


def get_ptau_url(log2: int) -> str:
    """Get the PTAU download URL for a given log2 size."""
    return f"{PTAU_BASE_URL}/powersOfTau28_hez_final_{log2:02d}.ptau"


def get_ptau_filename(log2: int) -> str:
    """Get the PTAU filename for a given log2 size."""
    return f"powersOfTau28_hez_final_{log2:02d}.ptau"


def download_ptau(ptau_path: Path, log2: int) -> None:
    """Download the Powers of Tau file if not present."""
    if ptau_path.exists():
        print(f"PTAU file already exists: {ptau_path}")
        return

    url = get_ptau_url(log2)
    print(f"Downloading PTAU from {url}...")
    if log2 >= 20:
        print("(This is a large file, may take a while...)")
    urllib.request.urlretrieve(url, ptau_path)
    print(f"Downloaded to {ptau_path}")


def build_binary(force: bool = False) -> None:
    """Build the Go binary if not present (or force rebuild)."""
    if BINARY.exists() and not force:
        print(f"Binary already exists: {BINARY}")
        return

    print("Building Go binary...")
    run_cmd(["go", "build"], cwd=REPO_ROOT)
    if not BINARY.exists():
        raise RuntimeError(f"Binary not found after build: {BINARY}")


def generate_r1cs(r1cs_path: Path) -> None:
    """Generate the R1CS file by running the Go test."""
    if r1cs_path.exists():
        print(f"R1CS file already exists: {r1cs_path}")
        return

    print("Generating R1CS from MiMC circuit...")
    run_cmd(["go", "test", "-v", "-run", "TestGenerateR1CS", "./test/"], cwd=REPO_ROOT)


def phase1_import(ptau_path: Path, phase1_path: Path, env: dict | None = None) -> None:
    """Import phase1 from PTAU file."""
    if phase1_path.exists():
        print(f"Phase1 already exists: {phase1_path}")
        return

    print("Importing phase1 from PTAU...")
    run_cmd([str(BINARY), "p1i", str(ptau_path), str(phase1_path)], env=env)


def phase2_init(
    phase1_path: Path,
    circuit_path: Path,
    phase2_path: Path,
    evals_path: Path,
    beacon_round: int,
    env: dict | None = None,
) -> None:
    """Initialize phase2."""
    if phase2_path.exists():
        print(f"Phase2 already exists: {phase2_path}")
        return

    if beacon_round <= 0:
        raise ValueError("beacon_round must be set to a positive drand round")

    print("Initializing phase2...")
    cmd = [str(BINARY), "p2n"]
    cmd.extend(["--beacon-round", str(beacon_round)])
    cmd.extend([
        str(phase1_path),
        str(circuit_path),
        str(phase2_path),
        str(evals_path),
    ])
    run_cmd(cmd, env=env)


def phase2_upload(bucket_name: str, env: dict | None = None) -> None:
    """Upload initial phase2 to S3.

    Note: p2u hardcodes path ./trusted-setup/phase2, so must run from repo root.
    """
    print(f"Uploading phase2 to S3 bucket: {bucket_name}...")
    run_cmd([str(BINARY), "p2u", bucket_name], cwd=REPO_ROOT, env=env)


def generate_presigned_urls(bucket_name: str, count: int, env: dict | None = None) -> list[tuple[int, str]]:
    """Generate presigned URLs and parse the output."""
    print(f"Generating {count} presigned URLs...")
    result = run_cmd([str(BINARY), "presigned", bucket_name, str(count)], capture_output=True, env=env)

    urls = []
    for line in result.stdout.strip().split("\n"):
        if not line:
            continue
        # Format: "0: https://..."
        index_str, url = line.split(": ", 1)
        urls.append((int(index_str), url))

    return urls


def phase2_contribute(presigned_url: str, bucket_name: str, env: dict | None = None) -> str:
    """Make a phase2 contribution. Returns the contribution hash.

    Note: p2c uses relative paths ./trusted-setup/, so must run from repo root.
    """
    print(f"Making contribution...")
    result = run_cmd([str(BINARY), "p2c", presigned_url, bucket_name], cwd=REPO_ROOT, capture_output=True, env=env)

    # Parse the contribution hash from output
    # Output contains: " - Contribution Hash: <hash>"
    for line in result.stdout.split("\n"):
        if "Contribution Hash:" in line:
            return line.split(":")[-1].strip()

    print(result.stdout)
    raise RuntimeError("Could not find contribution hash in output")


def phase2_verify(index: int, bucket_name: str, env: dict | None = None) -> None:
    """Verify a phase2 contribution.

    Note: p2v uses relative paths ./trusted-setup/, so must run from repo root.
    """
    print(f"Verifying contribution {index}...")
    run_cmd([str(BINARY), "p2v", str(index), bucket_name], cwd=REPO_ROOT, env=env)


def extract_keys(
    phase1_path: Path,
    phase2_path: Path,
    evals_path: Path,
    circuit_path: Path,
    env: dict | None = None,
) -> None:
    """Extract proving and verifying keys.

    Keys are output to current working directory (pk, vk files).
    """
    if phase1_beacon_round <= 0 or phase2_beacon_round <= 0:
        raise ValueError("phase1_beacon_round and phase2_beacon_round must be set to positive drand rounds")

    print("Extracting keys...")
    cmd = [str(BINARY), "key"]
    cmd.extend([
        str(phase1_path),
        str(phase2_path),
        str(evals_path),
        str(circuit_path),
    ])
    run_cmd(cmd, cwd=REPO_ROOT, env=env)


def generate_messages(bucket_name: str, urls: list[tuple[int, str]], messages_dir: Path) -> None:
    """Generate contributor message files.

    URL index 0 is for first contributor (uploads phase2-0, downloads phase2)
    URL index 1 is for second contributor (uploads phase2-1, downloads phase2-0)
    etc.
    """
    messages_dir.mkdir(parents=True, exist_ok=True)

    for index, url in urls:
        # Contributor number is index + 1 (1-indexed for human readability)
        contributor_num = index + 1
        msg_path = messages_dir / f"msg{contributor_num}.txt"
        msg_content = MESSAGE_TEMPLATE.format(
            index=contributor_num,
            url=url,
            bucket_name=bucket_name,
        )
        msg_path.write_text(msg_content)
        print(f"Created: {msg_path}")
