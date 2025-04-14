package test

import (
	"os"
	"strconv"
	"testing"

	curve "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	native_mimc "github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
	"github.com/consensys/gnark/backend/groth16"
	"github.com/consensys/gnark/backend/groth16/bn254/mpcsetup"
	cs "github.com/consensys/gnark/constraint/bn254"
	"github.com/consensys/gnark/frontend"
	deserializer "github.com/worldcoin/ptau-deserializer/deserialize"
)

type Config struct {
	PtauPath                          string
	Phase1OutputPath                  string
	Phase2OutputPath                  string
	Phase2WithContributionsOutputPath string
	EvalsOutputPath                   string
	R1csPath                          string
	NContributionsPhase2              int
	PkOutputPath                      string
	VkOutputPath                      string
	Power                             int
}

func TestEndToEnd(t *testing.T) {
	config := Config{
		PtauPath:                          "../build/powersOfTau28_hez_final_09.ptau",
		Phase1OutputPath:                  "../build/phase1",
		Phase2OutputPath:                  "../build/phase2",
		Phase2WithContributionsOutputPath: "../build/contributions",
		EvalsOutputPath:                   "../build/evals",
		R1csPath:                          "../build/r1cs",
		NContributionsPhase2:              3,
		PkOutputPath:                      "../build/pk",
		VkOutputPath:                      "../build/vk",
	}

	r1csFile, err := os.Open(config.R1csPath)
	if err != nil {
		panic(err)
	}
	r1cs := cs.R1CS{}
	r1cs.ReadFrom(r1csFile)

	ptau, err := deserializer.ReadPtau(config.PtauPath)
	if err != nil {
		panic(err)
	}

	phase1, err := deserializer.ConvertPtauToPhase1(ptau)
	if err != nil {
		panic(err)
	}

	phase1File, err := os.Create(config.Phase1OutputPath)
	if err != nil {
		panic(err)
	}

	_, err = phase1.WriteTo(phase1File)
	if err != nil {
		panic(err)
	}

	phase2, evals := mpcsetup.InitPhase2(&r1cs, &phase1)

	phase2File, err := os.Create(config.Phase2OutputPath)
	if err != nil {
		panic(err)
	}
	phase2.WriteTo(phase2File)

	evalsFile, err := os.Create(config.EvalsOutputPath)
	if err != nil {
		panic(err)
	}
	evals.WriteTo(evalsFile)

	for i := 0; i < config.NContributionsPhase2; i++ {
		prev := ClonePhase2(&phase2)
		phase2.Contribute()
		mpcsetup.VerifyPhase2(&prev, &phase2)
		phase2WithContributionFile, err := os.Create(config.Phase2WithContributionsOutputPath + "/contribution-" + strconv.Itoa(i))
		if err != nil {
			panic(err)
		}
		phase2.WriteTo(phase2WithContributionFile)
		phase2WithContributionFile.Close()
	}

	pk, vk := mpcsetup.ExtractKeys(&phase1, &phase2, &evals, r1cs.GetNbConstraints())

	pkFile, err := os.Create(config.PkOutputPath)
	if err != nil {
		panic(err)
	}
	pk.WriteTo(pkFile)
	pkFile.Close()

	vkFile, err := os.Create(config.VkOutputPath)
	if err != nil {
		panic(err)
	}
	vk.WriteTo(vkFile)
	vkFile.Close()

	// Build the witness
	var preImage, hash fr.Element
	{
		m := native_mimc.NewMiMC()
		m.Write(preImage.Marshal())
		hash.SetBytes(m.Sum(nil))
	}

	witness, err := frontend.NewWitness(&Circuit{PreImage: preImage, Hash: hash}, curve.ID.ScalarField())
	if err != nil {
		panic(err)
	}

	pubWitness, err := witness.Public()
	if err != nil {
		panic(err)
	}

	// groth16: ensure proof is verified
	proof, err := groth16.Prove(&r1cs, &pk, witness)
	if err != nil {
		panic(err)
	}

	err = groth16.Verify(proof, &vk, pubWitness)
	if err != nil {
		panic(err)
	}
}
