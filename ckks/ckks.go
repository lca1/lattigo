//Package ckks implements a RNS-accelerated version of the Homomorphic Encryption for Arithmetic for Approximate Numbers
//(HEAAN, a.k.a. CKKS) scheme. It provides approximate arithmetic over the complex numbers.
package ckks

import (
	"github.com/ldsec/lattigo/ring"
	"math/big"
)

// GaloisGen is an integer of order N/2 modulo M and that spans Z_M with the integer -1. The j-th ring automorphism takes the root zeta to zeta^(5j).
// Any other integer or order N/2 modulo M and congruent with 1 modulo 4 could be used instead.
const GaloisGen uint64 = 5

// Context is a struct that contains all the elements required to instantiate the CKKS Scheme. This includes the parameters (polynomial degree, ciphertext modulus,
// Gaussian sampler, polynomial contexts and other parameters required for the homomorphic operations).
type Context struct {

	// Context parameters
	logN     uint64
	logQ     uint64
	scale    float64
	n        uint64
	maxSlots uint64

	// Number of available levels
	levels uint64

	bigintChain []*big.Int

	// Contexts
	contextQ  *ring.Context
	contextP  *ring.Context
	contextQP *ring.Context

	// Samplers
	gaussianSampler *ring.KYSampler

	// Rotation params
	galElConjugate   uint64
	galElRotColLeft  []uint64
	galElRotColRight []uint64
}

// NewContext creates a new Context with the given parameters. It returns an error if one of the parameters would not ensure the
// correctness of the scheme (but it does not check for security).
func newContext(params *Parameters) (ckkscontext *Context) {

	if !params.isValid {
		panic("cannot newContext: parameters are invalid (check if the generation was done properly)")
	}

	var err error

	ckkscontext = new(Context)

	ckkscontext.logN = uint64(params.LogN)
	ckkscontext.n = 1 << uint64(params.LogN)
	ckkscontext.maxSlots = 1 << (uint64(params.LogN) - 1)
	ckkscontext.scale = params.Scale

	ckkscontext.levels = uint64(len(params.Qi))

	N := ckkscontext.n

	ckkscontext.bigintChain = genBigIntChain(params.Qi)

	if ckkscontext.contextQ, err = ring.NewContextWithParams(N, params.Qi); err != nil {
		panic(err)
	}

	if len(params.Pi) != 0 {
		if ckkscontext.contextP, err = ring.NewContextWithParams(N, params.Pi); err != nil {
			panic(err)
		}
	}

	if ckkscontext.contextQP, err = ring.NewContextWithParams(N, append(params.Qi, params.Pi...)); err != nil {
		panic(err)
	}

	ckkscontext.gaussianSampler = ckkscontext.contextQP.NewKYSampler(params.Sigma, int(6*params.Sigma))

	ckkscontext.galElRotColLeft = ring.GenGaloisParams(N, GaloisGen)
	ckkscontext.galElRotColRight = ring.GenGaloisParams(N, ring.ModExp(GaloisGen, 2*N-1, 2*N))
	ckkscontext.galElConjugate = 2*N - 1

	return ckkscontext

}
