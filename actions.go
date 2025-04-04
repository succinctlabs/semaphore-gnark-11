package main

import (
	"errors"
	"os"

	groth16 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/backend/groth16/bn254/mpcsetup"
	"github.com/consensys/gnark/backend/solidity"
	cs "github.com/consensys/gnark/constraint/bn254"
	"github.com/urfave/cli/v2"
	deserializer "github.com/worldcoin/ptau-deserializer/deserialize"
)

func p1i(cCtx *cli.Context) error {
	// sanity check
	if cCtx.Args().Len() != 2 {
		return errors.New("please provide the correct arguments")
	}

	ptauFilePath := cCtx.Args().Get(0)
	outputFilePath := cCtx.Args().Get(1)

	ptau, err := deserializer.ReadPtau(ptauFilePath)
	if err != nil {
		return err
	}

	phase1, err := deserializer.ConvertPtauToPhase1(ptau)
	if err != nil {
		return err
	}

	outputFile, err := os.Create(outputFilePath)
	if err != nil {
		return err
	}

	_, err = phase1.WriteTo(outputFile)
	if err != nil {
		return err
	}

	return nil
}

func p2n(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 4 {
		return errors.New("please provide the correct arguments")
	}

	phase1Path := cCtx.Args().Get(0)
	r1csPath := cCtx.Args().Get(1)
	phase2Path := cCtx.Args().Get(2)
	evalsPath := cCtx.Args().Get(3)

	phase1File, err := os.Open(phase1Path)
	if err != nil {
		return err
	}
	phase1 := &mpcsetup.Phase1{}
	phase1.ReadFrom(phase1File)

	r1csFile, err := os.Open(r1csPath)
	if err != nil {
		return err
	}
	r1cs := cs.R1CS{}
	r1cs.ReadFrom(r1csFile)

	phase2, evals := mpcsetup.InitPhase2(&r1cs, phase1)

	phase2File, err := os.Create(phase2Path)
	if err != nil {
		return err
	}
	phase2.WriteTo(phase2File)

	evalsFile, err := os.Create(evalsPath)
	if err != nil {
		return err
	}
	evals.WriteTo(evalsFile)

	return nil
}

func p2c(cCtx *cli.Context) error {
	inputPh2Path := cCtx.Args().Get(0)
	outputPh2Path := cCtx.Args().Get(1)

	inputFile, err := os.Open(inputPh2Path)
	if err != nil {
		return err
	}
	phase2 := &mpcsetup.Phase2{}
	phase2.ReadFrom(inputFile)

	phase2.Contribute()

	outputFile, err := os.Create(outputPh2Path)
	if err != nil {
		return err
	}
	phase2.WriteTo(outputFile)

	return nil
}

func p2v(cCtx *cli.Context) error {
	if cCtx.Args().Len() != 2 {
		return errors.New("please provide the correct arguments")
	}
	inputPath := cCtx.Args().Get(0)
	originPath := cCtx.Args().Get(1)

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	input := &mpcsetup.Phase2{}
	input.ReadFrom(inputFile)

	originFile, err := os.Open(originPath)
	if err != nil {
		return err
	}
	origin := &mpcsetup.Phase2{}
	origin.ReadFrom(originFile)

	mpcsetup.VerifyPhase2(origin, input)

	return nil
}

func keys(cCtx *cli.Context) error {
	// sanity check
	if cCtx.Args().Len() != 4 {
		return errors.New("please provide the correct arguments")
	}

	phase1Path := cCtx.Args().Get(0)
	phase1 := &mpcsetup.Phase1{}
	phase1File, err := os.Open(phase1Path)
	if err != nil {
		return err
	}
	phase1.ReadFrom(phase1File)

	phase2Path := cCtx.Args().Get(1)
	phase2 := &mpcsetup.Phase2{}
	phase2File, err := os.Open(phase2Path)
	if err != nil {
		return err
	}
	phase2.ReadFrom(phase2File)

	evalsPath := cCtx.Args().Get(2)
	evals := &mpcsetup.Phase2Evaluations{}
	evalsFile, err := os.Open(evalsPath)
	if err != nil {
		return err
	}
	evals.ReadFrom(evalsFile)

	r1csPath := cCtx.Args().Get(3)
	r1cs := &cs.R1CS{}
	r1csFile, err := os.Open(r1csPath)
	if err != nil {
		return err
	}
	r1cs.ReadFrom(r1csFile)

	// get number of constraints
	nbConstraints := r1cs.GetNbConstraints()

	pk, vk := mpcsetup.ExtractKeys(phase1, phase2, evals, nbConstraints)

	// write the proving key
	pkFile, err := os.Create("pk")
	if err != nil {
		return err
	}
	defer pkFile.Close()
	if err = pk.WriteDump(pkFile); err != nil {
		return err
	}

	vkFile, err := os.Create("vk")
	if err != nil {
		return err
	}
	vk.WriteTo(vkFile)

	return nil
}

func sol(cCtx *cli.Context) error {
	// sanity check
	if cCtx.Args().Len() != 1 {
		return errors.New("please provide the correct arguments")
	}

	vkPath := cCtx.Args().Get(0)
	vk := &groth16.VerifyingKey{}
	vkFile, err := os.Open(vkPath)
	if err != nil {
		return err
	}
	vk.ReadFrom(vkFile)

	solFile, err := os.Create("Groth16Verifier.sol")
	if err != nil {
		return err
	}

	err = vk.ExportSolidity(solFile, solidity.WithPragmaVersion("0.8.20"))
	return err
}

func ClonePhase1(phase1 *mpcsetup.Phase1) mpcsetup.Phase1 {
	r := mpcsetup.Phase1{}
	r.Parameters.G1.Tau = append(r.Parameters.G1.Tau, phase1.Parameters.G1.Tau...)
	r.Parameters.G1.AlphaTau = append(r.Parameters.G1.AlphaTau, phase1.Parameters.G1.AlphaTau...)
	r.Parameters.G1.BetaTau = append(r.Parameters.G1.BetaTau, phase1.Parameters.G1.BetaTau...)

	r.Parameters.G2.Tau = append(r.Parameters.G2.Tau, phase1.Parameters.G2.Tau...)
	r.Parameters.G2.Beta = phase1.Parameters.G2.Beta

	r.PublicKeys = phase1.PublicKeys
	r.Hash = append(r.Hash, phase1.Hash...)

	return r
}

func ClonePhase2(phase2 *mpcsetup.Phase2) mpcsetup.Phase2 {
	r := mpcsetup.Phase2{}
	r.Parameters.G1.Delta = phase2.Parameters.G1.Delta
	r.Parameters.G1.L = append(r.Parameters.G1.L, phase2.Parameters.G1.L...)
	r.Parameters.G1.Z = append(r.Parameters.G1.Z, phase2.Parameters.G1.Z...)
	r.Parameters.G2.Delta = phase2.Parameters.G2.Delta
	r.PublicKey = phase2.PublicKey
	r.Hash = append(r.Hash, phase2.Hash...)

	return r
}
