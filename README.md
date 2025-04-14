# Guide to the Succinct Trusted Setup Contribution Ceremony

This tool allows users to run an MPC ceremony for generating the proving and verifying keys for the Groth16/PLONK circuits used by Succinct [SP1](https://docs.succinct.xyz/docs/sp1/introduction).

This repo is adapted from Worldcoin's [Semaphore Merkle Tree Batcher](http://github.com/worldcoin/semaphore-mtb/).

## Reasoning behind a custom trusted setup

Each groth16 proof of a circuit requires a trusted setup that has 2 parts: a phase 1 which is also known as a "Powers of Tau ceremony" which is universal (the same one can be used for any circuit) and a phase2 which is circuit-specific, meaning that you need to a separate phase2 for every single circuit. In order to create an SRS to generate verifying keys for SMTB we would like many different members from different organizations to participate in the phase 2 of the trusted setup.

For the phase 1 we will be reusing the setup done by the joint effort of many community members, it is a powers of tau ceremony with 54 different contributions ([more info here](https://github.com/privacy-scaling-explorations/perpetualpowersoftau)). A list of downloadable `.ptau` files can be found [here](https://github.com/iden3/snarkjs/blob/master/README.md#7-prepare-phase-2).

## Pre-requisites

1. Install git https://github.com/git-guides/install-git
2. Install Go https://go.dev/doc/install
3. Minimum RAM requirement is 16GB

## Phase 2

In the phase 2, participants add randomness to make the setup specific to the actual circuit, creating the final proving and verification keys needed for SP1. To add your contribution to the phase 2, follow the steps below:

### Clone the repository and compile the program

```bash
git clone https://github.com/succinctlabs/semaphore-gnark-11.git
cd semaphore-gnark-11
go build
mkdir trusted-setup
```

### Run the contribution program to add your contribution

You will need two pieces of information provided by the coordinator:

* A Presigned URL: This is a special, temporary URL that grants permission to upload your contribution. It will look like a long web address.
* The S3 Bucket Name: The name of the cloud storage bucket where the ceremony files are stored.

Once you have received these 2 pieces of information, you can run the program below:

```bash
# Make sure to add quotes around the presigned URL to avoid `&` character in the URL being interpreted by your shell
# The command below can take around 10-20 minutes to complete
./semaphore-gnark-11 p2c "<presignedUrl>" <bucketName>
```

The output should look like this:

```
Downloading previous contribution: phase2-0
Generating contribution
Uploading contribution: phase2-1
Contribution successful!
Once your contribution has been verified by the coordinator, you can attest for it on social media, providing the following info:
 - Contribution URL: https://succinct-sp1-dev.s3.us-east-2.amazonaws.com/phase2-1
 - Contribution Hash: db0fbfa74ace3839c07d63041355754131dbfaececfbf64638f51e693455de8d
```

Then you can inform the coordinator that you have added your contribution, by providing them with the hash returned by the program, so they can verify it.

Once the coordinator has verified your contribution, you can publish an attestation for it on social media, specifying the URL and hash of your contribution.

## Verifying contribution

If you want, you can verify any contribution given its index. Run the following command:

```bash
./semaphore-gnark-11 p2v <index> <bucketName>
```

The output should look like this:

```
Downloading current contribution: phase2-1
Downloading phase2
Verifying contribution with hash: db0fbfa74ace3839c07d63041355754131dbfaececfbf64638f51e693455de8d
Ok!
```

## Acknowledgements

This repository is a fork of the [zkbnb-setup](https://github.com/bnb-chain/zkbnb-setup/) repository. We would like to thank the authors of the original repository for their work as this project is a slight tweak of the original work to fit our needs.

We appreciate the community efforts to generate [a good universal SRS](https://github.com/privacy-scaling-explorations/perpetualpowersoftau) for everyone's benefit to use and for the [iden3 team for building [snarkjs](https://github.com/iden3/snarkjs).

Also a special thank you to [Kobi Gurkan](https://twitter.com/kobigurk) for his contributions to the [ptau-deserialzier](https://github.com/worldcoin/ptau-deserializer) tool and his advice on the trusted setup process.
