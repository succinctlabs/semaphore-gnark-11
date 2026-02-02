# Practical guide for running a trusted setup ceremony for SP1 Groth16 Circuit.

## Prerequisites

- Go 1.23+
- Docker and docker-compose (for the e2e test)
- Python 3.10+ and [uv](https://github.com/astral-sh/uv) (for the e2e test)
- **Drand beacons**: Tests use hardcoded drand beacon rounds for deterministic, reproducible key generation
- A clone of sp1. Last tested on (this commit)[https://github.com/succinctlabs/sp1/tree/3800bc387db4fe2c47cbc5055a364732a5f3e3dd]. Make sure you can build sp1 from source.

## Build the circuit

From the sp1 repo root, run

```bash
cd crates/prover
make build-circuits
```

This will generate the circuit files in `crates/prover/build/groth16`.

## Start the trusted setup ceremony.

From this repo root, first make sure `build` and `trusted-setup` are cleared. Then run:

```bash
bash scripts/create-public-s3-bucket.sh <bucket-name>
go build
python3 scripts/initialize_trusted_setup.py <bucket-name> --circuit-path <circuit-path>/groth16_circuit.bin  --contribution-count <num-contributors> --phase1-beacon-round <first-beacon-round> 
```

where: 
- `<bucket-name>` is the name of the S3 bucket to use for the ceremony. 
- `<circuit-path>` is the path to the circuit file generated in the previous step. It's `<sp1-repo-root>/crates/prover/build/groth16_circuit.bin`.
- `<num-contributors>` is the number of contributors to the ceremony.
- `<first-beacon-round>` is the drand beacon round to use for the first contribution.

This script will output some messages in `trusted-setup/messages` directory. Send them to the people you want to contribute to the ceremony.

## Ceremony rounds

1. Wait for the first contribution to be uploaded to the S3 bucket.
2. Verify it with this
`./semaphore-gnark-11 p2v 0 <bucket-name>`
Repeat this step for each contribution.

## Finalize the ceremony

1. Run the finalization script:
```bash
bash scripts/finalize.sh <first-beacon-round> <last-beacon-round> <circuit-path>
```

This will generate the keys, create the Solidity verifier, and archive the trusted-setup directory.

Then finish up by uploading everything into the s3 bucket. Scripts are in sp1/crates/prover
