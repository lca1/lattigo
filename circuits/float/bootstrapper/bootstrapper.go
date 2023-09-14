// Package bootstrapper implements the Bootstrapper struct which provides generic bootstrapping for the CKKS scheme (and RLWE ciphertexts by extension).
// It notably abstracts scheme switching and ring dimension switching, enabling efficient bootstrapping of ciphertexts in the Conjugate Invariant ring
// or multiple ciphertexts of a lower ring dimension.
package bootstrapper

import (
	"fmt"
	"math/big"
	"runtime"

	"github.com/tuneinsight/lattigo/v4/circuits/float/bootstrapper/bootstrapping"
	"github.com/tuneinsight/lattigo/v4/ckks"
	"github.com/tuneinsight/lattigo/v4/ring"
	"github.com/tuneinsight/lattigo/v4/rlwe"
)

type Bootstrapper struct {
	bridge       ckks.DomainSwitcher
	bootstrapper *bootstrapping.Bootstrapper

	paramsN1    ckks.Parameters
	paramsN2    ckks.Parameters
	btpParamsN2 bootstrapping.Parameters

	xPow2N1    []ring.Poly
	xPow2InvN1 []ring.Poly
	xPow2N2    []ring.Poly
	xPow2InvN2 []ring.Poly

	evk BootstrappingKeys
}

type BootstrappingKeys struct {
	EvkN1ToN2        *rlwe.EvaluationKey
	EvkN2ToN1        *rlwe.EvaluationKey
	EvkRealToCmplx   *rlwe.EvaluationKey
	EvkCmplxToReal   *rlwe.EvaluationKey
	EvkBootstrapping *bootstrapping.EvaluationKeySet
}

func (b BootstrappingKeys) BinarySize() (dLen int) {
	if b.EvkN1ToN2 != nil {
		dLen += b.EvkN1ToN2.BinarySize()
	}

	if b.EvkN2ToN1 != nil {
		dLen += b.EvkN2ToN1.BinarySize()
	}

	if b.EvkRealToCmplx != nil {
		dLen += b.EvkRealToCmplx.BinarySize()
	}

	if b.EvkCmplxToReal != nil {
		dLen += b.EvkCmplxToReal.BinarySize()
	}

	if b.EvkBootstrapping != nil {
		dLen += b.EvkBootstrapping.BinarySize()
	}

	return
}

func GenBootstrappingKeys(paramsN1 ckks.Parameters, btpParamsN2 bootstrapping.Parameters, skN1 *rlwe.SecretKey) (BootstrappingKeys, error) {

	var EvkN1ToN2, EvkN2ToN1 *rlwe.EvaluationKey
	var EvkRealToCmplx *rlwe.EvaluationKey
	var EvkCmplxToReal *rlwe.EvaluationKey
	paramsN2 := btpParamsN2.Parameters

	// Checks that the maximum level of paramsN1 is equal to the remaining level after the bootstrapping of paramsN2
	if paramsN2.MaxLevel()-btpParamsN2.SlotsToCoeffsParameters.Depth(true)-btpParamsN2.Mod1ParametersLiteral.Depth()-btpParamsN2.CoeffsToSlotsParameters.Depth(true) < paramsN1.MaxLevel() {
		return BootstrappingKeys{}, fmt.Errorf("cannot GenBootstrappingKeys: bootstrapping depth is too large, level after bootstrapping is smaller than paramsN1.MaxLevel()")
	}

	// Checks that the overlapping primes between paramsN1 and paramsN2 are the same, i.e.
	// pN1: q0, q1, q2, ..., qL
	// pN2: q0, q1, q2, ..., qL, [bootstrapping primes]
	QN1 := paramsN1.Q()
	QN2 := paramsN2.Q()

	for i := range QN1 {
		if QN1[i] != QN2[i] {
			return BootstrappingKeys{}, fmt.Errorf("cannot GenBootstrappingKeys: paramsN1.Q() is not a subset of paramsN2.Q()")
		}
	}

	kgen := ckks.NewKeyGenerator(paramsN2)

	// Ephemeral secret-key used to generate the evaluation keys.
	skN2 := rlwe.NewSecretKey(paramsN2)
	buff := paramsN2.RingQ().NewPoly()
	ringQ := paramsN2.RingQ()
	ringP := paramsN2.RingP()

	switch paramsN1.RingType() {
	// In this case we need need generate the bridge switching keys between the two rings
	case ring.ConjugateInvariant:

		if paramsN1.LogN() != paramsN2.LogN()-1 {
			return BootstrappingKeys{}, fmt.Errorf("cannot GenBootstrappingKeys: if paramsN1.RingType() == ring.ConjugateInvariant then must ensure that paramsN1.LogN()+1 == paramsN2.LogN()-1")
		}

		// R[X+X^-1]/(X^N +1) -> R[X]/(X^2N + 1)
		ringQ.AtLevel(skN1.LevelQ()).UnfoldConjugateInvariantToStandard(skN1.Value.Q, skN2.Value.Q)

		// Extends basis Q0 -> QL
		rlwe.ExtendBasisSmallNormAndCenterNTTMontgomery(ringQ, ringQ, skN2.Value.Q, buff, skN2.Value.Q)

		// Extends basis Q0 -> P
		rlwe.ExtendBasisSmallNormAndCenterNTTMontgomery(ringQ, ringP, skN2.Value.Q, buff, skN2.Value.P)

		EvkCmplxToReal, EvkRealToCmplx = kgen.GenEvaluationKeysForRingSwapNew(skN2, skN1)

	// Only regular key-switching is required in this case
	case ring.Standard:

		// Maps the smaller key to the largest with Y = X^{N/n}.
		ring.MapSmallDimensionToLargerDimensionNTT(skN1.Value.Q, skN2.Value.Q)

		// Extends basis Q0 -> QL
		rlwe.ExtendBasisSmallNormAndCenterNTTMontgomery(ringQ, ringQ, skN2.Value.Q, buff, skN2.Value.Q)

		// Extends basis Q0 -> P
		rlwe.ExtendBasisSmallNormAndCenterNTTMontgomery(ringQ, ringP, skN2.Value.Q, buff, skN2.Value.P)

		EvkN1ToN2 = kgen.GenEvaluationKeyNew(skN1, skN2)
		EvkN2ToN1 = kgen.GenEvaluationKeyNew(skN2, skN1)
	}

	return BootstrappingKeys{
		EvkN1ToN2:        EvkN1ToN2,
		EvkN2ToN1:        EvkN2ToN1,
		EvkRealToCmplx:   EvkRealToCmplx,
		EvkCmplxToReal:   EvkCmplxToReal,
		EvkBootstrapping: btpParamsN2.GenEvaluationKeySetNew(skN2),
	}, nil
}

func NewBootstrapper(paramsN1 ckks.Parameters, btpParamsN2 bootstrapping.Parameters, evk BootstrappingKeys) (rlwe.Bootstrapper, error) {

	b := &Bootstrapper{}

	paramsN2 := btpParamsN2.Parameters

	switch paramsN1.RingType() {
	case ring.Standard:
		if evk.EvkN1ToN2 == nil || evk.EvkN2ToN1 == nil {
			return nil, fmt.Errorf("cannot NewBootstrapper: evk.(BootstrappingKeys) is missing EvkN1ToN2 and EvkN2ToN1")
		}
	case ring.ConjugateInvariant:
		if evk.EvkCmplxToReal == nil || evk.EvkRealToCmplx == nil {
			return nil, fmt.Errorf("cannot NewBootstrapper: evk.(BootstrappingKeys) is missing EvkN1ToN2 and EvkN2ToN1")
		}

		var err error
		if b.bridge, err = ckks.NewDomainSwitcher(paramsN2, evk.EvkCmplxToReal, evk.EvkRealToCmplx); err != nil {
			return nil, fmt.Errorf("cannot NewBootstrapper: ckks.NewDomainSwitcher: %w", err)
		}

		// The switch to standard to conjugate invariant multiplies the scale by 2
		btpParamsN2.SlotsToCoeffsParameters.Scaling = new(big.Float).SetFloat64(0.5)
	}

	b.paramsN1 = paramsN1
	b.paramsN2 = paramsN2
	b.btpParamsN2 = btpParamsN2
	b.evk = evk

	b.xPow2N2 = rlwe.GenXPow2(b.paramsN2.RingQ().AtLevel(0), b.paramsN2.LogN(), false)
	b.xPow2InvN2 = rlwe.GenXPow2(b.paramsN2.RingQ(), b.paramsN2.LogN(), true)

	if paramsN1.N() != b.paramsN2.N() {
		b.xPow2N1 = b.xPow2N2
		b.xPow2InvN1 = b.xPow2InvN2
	} else {
		b.xPow2N1 = rlwe.GenXPow2(b.paramsN1.RingQ().AtLevel(0), b.paramsN2.LogN(), false)
		b.xPow2InvN1 = rlwe.GenXPow2(b.paramsN1.RingQ(), b.paramsN2.LogN(), true)
	}

	var err error
	if b.bootstrapper, err = bootstrapping.NewBootstrapper(btpParamsN2, evk.EvkBootstrapping); err != nil {
		return nil, err
	}

	return b, nil
}

func (b Bootstrapper) Depth() int {
	return b.btpParamsN2.SlotsToCoeffsParameters.Depth(true) + b.btpParamsN2.Mod1ParametersLiteral.Depth() + b.btpParamsN2.CoeffsToSlotsParameters.Depth(true)
}

func (b Bootstrapper) OutputLevel() int {
	return b.paramsN2.MaxLevel() - b.Depth()
}

func (b Bootstrapper) MinimumInputLevel() int {
	return 0
}

func (b Bootstrapper) Bootstrap(ct *rlwe.Ciphertext) (*rlwe.Ciphertext, error) {
	cts := []*rlwe.Ciphertext{ct}
	cts, err := b.BootstrapMany(cts)
	return cts[0], err
}

func (b Bootstrapper) BootstrapMany(cts []*rlwe.Ciphertext) ([]*rlwe.Ciphertext, error) {

	var err error

	switch b.paramsN1.RingType() {
	case ring.ConjugateInvariant:

		for i := 0; i < len(cts); i = i + 2 {

			even, odd := i, i+1

			ct0 := cts[even]

			var ct1 *rlwe.Ciphertext
			if odd < len(cts) {
				ct1 = cts[odd]
			}

			if ct0, ct1, err = b.refreshConjugateInvariant(ct0, ct1); err != nil {
				return nil, fmt.Errorf("cannot BootstrapMany: %w", err)
			}

			cts[even] = ct0

			if ct1 != nil {
				cts[odd] = ct1
			}
		}

	default:

		LogSlots := cts[0].LogSlots()
		nbCiphertexts := len(cts)

		if cts, err = b.PackAndSwitchN1ToN2(cts); err != nil {
			return nil, fmt.Errorf("cannot BootstrapMany: %w", err)
		}

		for i := range cts {
			var ct *rlwe.Ciphertext
			if ct, err = b.bootstrapper.Bootstrap(cts[i]); err != nil {
				return nil, fmt.Errorf("cannot BootstrapMany: %w", err)
			}
			cts[i] = ct
		}

		if cts, err = b.UnpackAndSwitchN2Tn1(cts, LogSlots, nbCiphertexts); err != nil {
			return nil, fmt.Errorf("cannot BootstrapMany: %w", err)
		}
	}

	runtime.GC()

	for i := range cts {
		cts[i].Scale = b.paramsN1.DefaultScale()
	}

	return cts, err
}

// refreshConjugateInvariant takes two ciphertext in the Conjugate Invariant ring, repacks them in a single ciphertext in the standard ring
// using the real and imaginary part, bootstrap both ciphertext, and then extract back the real and imaginary part before repacking them
// individually in two new ciphertexts in the Conjugate Invariant ring.
func (b Bootstrapper) refreshConjugateInvariant(ctLeftN1Q0, ctRightN1Q0 *rlwe.Ciphertext) (ctLeftN1QL, ctRightN1QL *rlwe.Ciphertext, err error) {

	if ctLeftN1Q0 == nil {
		panic("cannot refreshConjugateInvariant: ctLeftN1Q0 cannot be nil")
	}

	// Switches ring from ring.ConjugateInvariant to ring.Standard
	ctLeftN2Q0 := b.RealToComplexNew(ctLeftN1Q0)

	// Repacks ctRightN1Q0 into the imaginary part of ctLeftN1Q0
	// which is zero since it comes from the Conjugate Invariant ring)
	if ctRightN1Q0 != nil {
		ctRightN2Q0 := b.RealToComplexNew(ctRightN1Q0)

		if err = b.bootstrapper.Evaluator.Mul(ctRightN2Q0, 1i, ctRightN2Q0); err != nil {
			return nil, nil, fmt.Errorf("cannot BootstrapMany: %w", err)
		}

		if err = b.bootstrapper.Evaluator.Add(ctLeftN2Q0, ctRightN2Q0, ctLeftN2Q0); err != nil {
			return nil, nil, fmt.Errorf("cannot BootstrapMany: %w", err)
		}
	}

	// Refreshes in the ring.Sstandard
	var ctLeftAndRightN2QL *rlwe.Ciphertext
	if ctLeftAndRightN2QL, err = b.bootstrapper.Bootstrap(ctLeftN2Q0); err != nil {
		return nil, nil, fmt.Errorf("cannot BootstrapMany: %w", err)
	}

	// The SlotsToCoeffs transformation scales the ciphertext by 0.5
	// This is done to compensate for the 2x factor introduced by ringStandardToConjugate(*).
	ctLeftAndRightN2QL.Scale = ctLeftAndRightN2QL.Scale.Mul(rlwe.NewScale(1 / 2.0))

	// Switches ring from ring.Standard to ring.ConjugateInvariant
	ctLeftN1QL = b.ComplexToRealNew(ctLeftAndRightN2QL)

	// Extracts the imaginary part
	if ctRightN1Q0 != nil {
		if err = b.bootstrapper.Mul(ctLeftAndRightN2QL, -1i, ctLeftAndRightN2QL); err != nil {
			return nil, nil, fmt.Errorf("cannot BootstrapMany: %w", err)
		}
		ctRightN1QL = b.ComplexToRealNew(ctLeftAndRightN2QL)
	}

	return
}
