package test

import (
	"os"
	"reflect"
	"strconv"
	"testing"
	"unsafe"

	curve "github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	native_mimc "github.com/consensys/gnark-crypto/ecc/bn254/fr/mimc"
	"github.com/consensys/gnark/backend/groth16"
	groth16Impl "github.com/consensys/gnark/backend/groth16/bn254"
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

// getPhase1Commons extracts the unexported SrsCommons from Phase1 using reflection
func getPhase1Commons(phase1 *mpcsetup.Phase1) *mpcsetup.SrsCommons {
	v := reflect.ValueOf(phase1).Elem()
	params := v.FieldByName("parameters")
	return (*mpcsetup.SrsCommons)(unsafe.Pointer(params.UnsafeAddr()))
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

	// Get SrsCommons from Phase1 using reflection
	commons := getPhase1Commons(&phase1)

	// Initialize Phase2
	phase2 := &mpcsetup.Phase2{}
	evals := phase2.Initialize(&r1cs, commons)

	phase2File, err := os.Create(config.Phase2OutputPath)
	if err != nil {
		panic(err)
	}
	phase2.WriteTo(phase2File)

	// Store initial phase2 for verification
	initialPhase2 := &mpcsetup.Phase2{}
	initialPhase2.Initialize(&r1cs, commons)

	for i := 0; i < config.NContributionsPhase2; i++ {
		// Read the previous contribution
		prevFile, err := os.Open(config.Phase2OutputPath)
		if err != nil {
			panic(err)
		}
		prev := &mpcsetup.Phase2{}
		prev.ReadFrom(prevFile)
		prevFile.Close()

		phase2.Contribute()

		// Verify the contribution
		if err := prev.Verify(phase2); err != nil {
			panic(err)
		}

		phase2WithContributionFile, err := os.Create(config.Phase2WithContributionsOutputPath + "/contribution-" + strconv.Itoa(i))
		if err != nil {
			panic(err)
		}
		phase2.WriteTo(phase2WithContributionFile)
		phase2WithContributionFile.Close()

		// Update phase2 file for next iteration
		phase2File, err := os.Create(config.Phase2OutputPath)
		if err != nil {
			panic(err)
		}
		phase2.WriteTo(phase2File)
		phase2File.Close()
	}

	// Use a deterministic beacon challenge for key extraction
	beaconChallenge := []byte("test-beacon-challenge")

	pk, vk := phase2.Seal(commons, &evals, beaconChallenge)

	pkTyped := pk.(*groth16Impl.ProvingKey)
	vkTyped := vk.(*groth16Impl.VerifyingKey)

	pkFile, err := os.Create(config.PkOutputPath)
	if err != nil {
		panic(err)
	}
	pk.WriteDump(pkFile)
	pkFile.Close()

	vkFile, err := os.Create(config.VkOutputPath)
	if err != nil {
		panic(err)
	}
	vkTyped.WriteTo(vkFile)
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
	proof, err := groth16.Prove(&r1cs, pkTyped, witness)
	if err != nil {
		panic(err)
	}

	err = groth16.Verify(proof, vkTyped, pubWitness)
	if err != nil {
		panic(err)
	}
}
