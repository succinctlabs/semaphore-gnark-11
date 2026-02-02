package test

import (
	"os"
	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bn254"

	"github.com/consensys/gnark/backend/groth16"
	groth16_bn254 "github.com/consensys/gnark/backend/groth16/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
	"github.com/consensys/gnark/std/hash/mimc"
)

type Circuit struct {
	PreImage frontend.Variable
	Hash     frontend.Variable `gnark:",public"`
}

func (circuit *Circuit) Define(api frontend.API) error {
	mimc, _ := mimc.NewMiMC(api)

	mimc.Write(circuit.PreImage)
	api.AssertIsEqual(circuit.Hash, mimc.Sum())

	return nil
}

func TestProveAndVerifyV2(t *testing.T) {
	// Compile the circuit
	var myCircuit Circuit
	ccs, err := frontend.Compile(bn254.ID.ScalarField(), r1cs.NewBuilder, &myCircuit)
	if err != nil {
		t.Error(err)
	}

	// Read PK (WriteDump format) and VK
	pkk := &groth16_bn254.ProvingKey{}
	pkFile, _ := os.Open("../build/pk")
	defer pkFile.Close()
	pkk.ReadDump(pkFile)
	vkk := groth16.NewVerifyingKey(ecc.BN254)
	vkFile, _ := os.Open("../build/vk")
	defer vkFile.Close()
	vkk.ReadFrom(vkFile)

	assignment := &Circuit{
		PreImage: "16130099170765464552823636852555369511329944820189892919423002775646948828469",
		Hash:     "12886436712380113721405259596386800092738845035233065858332878701083870690753",
	}
	witness, _ := frontend.NewWitness(assignment, bn254.ID.ScalarField())
	prf, err := groth16.Prove(ccs, pkk, witness)
	if err != nil {
		panic(err)
	}
	pubWitness, err := witness.Public()
	if err != nil {
		panic(err)
	}
	err = groth16.Verify(prf, vkk, pubWitness)
	if err != nil {
		panic(err)
	}
}
