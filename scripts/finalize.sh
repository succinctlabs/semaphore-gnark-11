#!/bin/bash

# Finalize the trusted setup ceremony
# Usage: ./finalize.sh <first-beacon-round> <last-beacon-round> <circuit-path>
#
# Arguments:
#   first-beacon-round: The drand beacon round used for phase 1
#   last-beacon-round: The drand beacon round to use for phase 2
#   circuit-path: Path to the directory containing groth16_circuit.bin

set -e

if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <first-beacon-round> <last-beacon-round> <circuit-path>"
    echo ""
    echo "Example: $0 12345 12350 /path/to/sp1/crates/prover/build"
    exit 1
fi

FIRST_BEACON_ROUND=$1
LAST_BEACON_ROUND=$2
CIRCUIT_PATH=$3

# Validate circuit path
if [ ! -f "${CIRCUIT_PATH}/groth16_circuit.bin" ]; then
    echo "Error: Circuit file not found at ${CIRCUIT_PATH}/groth16_circuit.bin"
    exit 1
fi

echo "Finalizing trusted setup ceremony..."
echo "  Phase 1 beacon round: ${FIRST_BEACON_ROUND}"
echo "  Phase 2 beacon round: ${LAST_BEACON_ROUND}"
echo "  Circuit path: ${CIRCUIT_PATH}"
echo ""

# Generate the proving and verifying keys
echo "Generating keys..."
./semaphore-gnark-11 key --phase1-beacon-round ${FIRST_BEACON_ROUND} --phase2-beacon-round ${LAST_BEACON_ROUND} trusted-setup/phase1 trusted-setup/phase2-2 trusted-setup/evals ${CIRCUIT_PATH}/groth16_circuit.bin

# Copy keys to circuit directory
echo "Copying keys to circuit directory..."
cp pk ${CIRCUIT_PATH}/groth16_pk.bin
cp vk ${CIRCUIT_PATH}/groth16_vk.bin

# Generate Solidity verifier
echo "Generating Solidity verifier..."
./semaphore-gnark-11 sol vk

# Copy verifier and phase2 file
echo "Copying verifier and phase2 file..."
cp Groth16Verifier.sol trusted-setup/Groth16Verifier.sol
cp phase2 ${CIRCUIT_PATH}/groth16_phase2.bin

echo ""
echo "Finalization complete!"
echo "  Proving key: ${CIRCUIT_PATH}/groth16_pk.bin"
echo "  Verifying key: ${CIRCUIT_PATH}/groth16_vk.bin"
echo "  Phase 2 file: ${CIRCUIT_PATH}/groth16_phase2.bin"
echo "  Solidity verifier: trusted-setup/Groth16Verifier.sol"

# Archive the trusted setup directory
echo ""
echo "Creating archive of trusted-setup directory..."
tar -czf trusted-setup.tar.gz trusted-setup/
echo "Archive created: trusted-setup.tar.gz"
