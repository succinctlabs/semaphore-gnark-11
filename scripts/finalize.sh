#!/bin/bash

# Finalize the trusted setup ceremony
# Usage: ./finalize.sh <circuit-path>
#
# Reads beacon rounds from trusted-setup/beacon-rounds.txt (created during initialization)
#
# Arguments:
#   circuit-path: Path to the directory containing groth16_circuit.bin

set -e

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <circuit-path>"
    echo ""
    echo "Example: $0 /path/to/sp1/crates/prover/build"
    echo ""
    echo "Note: Beacon rounds are read from trusted-setup/beacon-rounds.txt"
    exit 1
fi

CIRCUIT_PATH=$1

# Validate circuit path
if [ ! -f "${CIRCUIT_PATH}/groth16_circuit.bin" ]; then
    echo "Error: Circuit file not found at ${CIRCUIT_PATH}/groth16_circuit.bin"
    exit 1
fi

# Check for beacon-rounds.txt
if [ ! -f "trusted-setup/beacon-rounds.txt" ]; then
    echo "Error: beacon-rounds.txt not found in trusted-setup/"
    echo "This file should have been created during initialization."
    exit 1
fi

echo "Finalizing trusted setup ceremony..."
echo "  Circuit path: ${CIRCUIT_PATH}"
echo "  Beacon rounds file: trusted-setup/beacon-rounds.txt"
echo ""

# Generate the proving and verifying keys
# The key command will read beacon rounds from trusted-setup/beacon-rounds.txt
echo "Generating keys..."
./semaphore-gnark-11 key trusted-setup/phase1 trusted-setup/phase2-2 trusted-setup/evals ${CIRCUIT_PATH}/groth16_circuit.bin

# # Copy keys to circuit directory
# echo "Copying keys to circuit directory..."
# cp pk ${CIRCUIT_PATH}/groth16_pk.bin
# cp vk ${CIRCUIT_PATH}/groth16_vk.bin

# Generate Solidity verifier
echo "Generating Solidity verifier..."
./semaphore-gnark-11 sol vk

# Copy verifier and phase2 file
echo "Copying verifier and phase2 file..."
cp Groth16Verifier.sol trusted-setup/Groth16Verifier.sol

echo ""
echo "Finalization complete!"
echo "  Proving key: ${CIRCUIT_PATH}/groth16_pk.bin"
echo "  Verifying key: ${CIRCUIT_PATH}/groth16_vk.bin"
echo "  Solidity verifier: trusted-setup/Groth16Verifier.sol"

# Archive the trusted setup directory
echo ""
echo "Creating archive of trusted-setup directory..."
mv trusted-setup/messages messages
tar -czf trusted-setup.tar.gz trusted-setup/
echo "Archive created: trusted-setup.tar.gz"
