package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:      "setup",
		Usage:     "Use this tool to generate parameters of Groth16 via MPC",
		UsageText: "setup command [arguments...]",
		Commands: []*cli.Command{

			/* ----------------------------- Phase 1 Import ----------------------------- */
			{
				Name:        "p1i",
				Usage:       "p1i <inputPtau> <outputPh1>",
				Description: "Deserialize snarkjs .ptau file into gnark's phase1 format and write to `OUTPUT`.ph1",
				Action:      p1i,
			},
			/* --------------------------- Phase 2 Initialize --------------------------- */
			{
				Name:        "p2n",
				Usage:       "p2n <inputPh1> <inputR1cs> <outputPh2> <outputEvals>",
				Description: "initialize phase 2 for the given circuit",
				Action:      p2n,
			},
			/* ----------------------------- Phase 2 Download from S3 ----------------------- */
			{
				Name:        "p2d",
				Usage:       "p2d <objectKey> <s3region> <s3bucket>",
				Description: "download a phase 2 contribution from an AWS S3 bucket",
				Action:      p2d,
			},
			/* --------------------------- Phase 2 Contribute --------------------------- */
			{
				Name:        "p2c",
				Usage:       "p2c <inputPh2> <outputPh2>",
				Description: "contribute phase 2 randomness for Groth16",
				Action:      p2c,
			},
			/* ----------------------------- Phase 2 Verify ----------------------------- */
			{
				Name:        "p2v",
				Usage:       "p2v <inputPath> <originPath>",
				Description: "verify phase 2 contributions for Groth16",
				Action:      p2v,
			},
			/* ----------------------------- Phase 2 Upload to S3 ----------------------- */
			{
				Name:        "p2u",
				Usage:       "p2u <inputPath> <s3region> <s3bucket>",
				Description: "upload a phase 2 contribution to an AWS S3 bucket",
				Action:      p2u,
			},
			/* ----------------------------- Keys Extraction ---------------------------- */
			{
				Name:        "key",
				Usage:       "key <phase1Path> <phase2Path> <phase2EvalsPath> <r1csPath>",
				Description: "extract proving and verifying keys",
				Action:      keys,
			},
			{
				Name:        "sol",
				Usage:       "sol <verifyingKey>",
				Description: "export verifier smart contract from verifying key",
				Action:      sol,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
