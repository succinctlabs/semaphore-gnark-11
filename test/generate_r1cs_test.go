package test

import (
	"os"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/r1cs"
)

// TestGenerateR1CS compiles the MiMC circuit and writes the R1CS to build/r1cs
func TestGenerateR1CS(t *testing.T) {
	if err := os.MkdirAll("../build", 0755); err != nil {
		t.Fatal(err)
	}

	var circuit Circuit
	ccs, err := frontend.Compile(bn254.ID.ScalarField(), r1cs.NewBuilder, &circuit)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Create("../build/r1cs")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	_, err = ccs.WriteTo(f)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("R1CS written to build/r1cs with %d constraints", ccs.GetNbConstraints())
}
