package dckks

import (
	"encoding/json"
	"flag"
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tuneinsight/lattigo/v3/ckks"
	"github.com/tuneinsight/lattigo/v3/drlwe"
	"github.com/tuneinsight/lattigo/v3/ring"
	"github.com/tuneinsight/lattigo/v3/rlwe"
	"github.com/tuneinsight/lattigo/v3/utils"
)

var flagLongTest = flag.Bool("long", false, "run the long test suite (all parameters + secure bootstrapping). Overrides -short and requires -timeout=0.")
var flagPostQuantum = flag.Bool("pq", false, "run post quantum test suite (does not run non-PQ parameters).")
var flagParamString = flag.String("params", "", "specify the test cryptographic parameters as a JSON string. Overrides -short and -long.")
var printPrecisionStats = flag.Bool("print-precision", false, "print precision stats")
var minPrec float64 = 15.0
var parties int = 3

func testString(opname string, parties int, params ckks.Parameters) string {
	return fmt.Sprintf("%s/RingType=%s/logN=%d/logSlots=%d/logQ=%d/levels=%d/#Pi=%d/Decomp=%d/parties=%d",
		opname,
		params.RingType(),
		params.LogN(),
		params.LogSlots(),
		params.LogQP(),
		params.MaxLevel()+1,
		params.PCount(),
		params.DecompRNS(params.QCount()-1, params.PCount()-1),
		parties)
}

type testContext struct {
	params ckks.Parameters

	ringQ *ring.Ring
	ringP *ring.Ring

	encoder   ckks.Encoder
	evaluator ckks.Evaluator

	encryptorPk0 ckks.Encryptor
	decryptorSk0 ckks.Decryptor
	decryptorSk1 ckks.Decryptor

	pk0 *rlwe.PublicKey
	pk1 *rlwe.PublicKey

	sk0 *rlwe.SecretKey
	sk1 *rlwe.SecretKey

	sk0Shards []*rlwe.SecretKey
	sk1Shards []*rlwe.SecretKey

	crs            drlwe.CRS
	uniformSampler *ring.UniformSampler
}

func TestDCKKS(t *testing.T) {

	var err error

	var testParams []ckks.ParametersLiteral
	switch {
	case *flagParamString != "": // the custom test suite reads the parameters from the -params flag
		testParams = append(testParams, ckks.ParametersLiteral{})
		if err = json.Unmarshal([]byte(*flagParamString), &testParams[0]); err != nil {
			t.Fatal(err)
		}
	case *flagLongTest:
		for _, pls := range [][]ckks.ParametersLiteral{
			ckks.DefaultParams,
			ckks.DefaultConjugateInvariantParams,
			ckks.DefaultPostQuantumParams,
			ckks.DefaultPostQuantumConjugateInvariantParams} {
			testParams = append(testParams, pls...)
		}
	case *flagPostQuantum && testing.Short():
		testParams = append(ckks.DefaultPostQuantumParams[:2], ckks.DefaultPostQuantumConjugateInvariantParams[:2]...)
	case *flagPostQuantum:
		testParams = append(ckks.DefaultPostQuantumParams[:4], ckks.DefaultPostQuantumConjugateInvariantParams[:4]...)
	case testing.Short():
		testParams = append(ckks.DefaultParams[:2], ckks.DefaultConjugateInvariantParams[:2]...)
	default:
		testParams = append(ckks.DefaultParams[:4], ckks.DefaultConjugateInvariantParams[:4]...)
	}

	for _, paramsLiteral := range testParams {

		var params ckks.Parameters
		if params, err = ckks.NewParametersFromLiteral(paramsLiteral); err != nil {
			t.Fatal(err)
		}

		var tc *testContext
		if tc, err = genTestParams(params); err != nil {
			t.Fatal(err)
		}

		for _, testSet := range []func(tc *testContext, t *testing.T){
			testPublicKeyGen,
			testRelinKeyGen,
			testKeyswitching,
			testPublicKeySwitching,
			testRotKeyGenConjugate,
			testRotKeyGenCols,
			testE2SProtocol,
			testRefresh,
			testRefreshAndTransform,
			testRefreshAndTransformSwitchParams,
			testMarshalling,
		} {
			testSet(tc, t)
			runtime.GC()
		}
	}
}

func genTestParams(params ckks.Parameters) (tc *testContext, err error) {

	tc = new(testContext)

	tc.params = params

	tc.ringQ = params.RingQ()
	tc.ringP = params.RingP()

	prng, _ := utils.NewKeyedPRNG([]byte{'t', 'e', 's', 't'})
	tc.crs = prng
	tc.uniformSampler = ring.NewUniformSampler(prng, params.RingQ())

	tc.encoder = ckks.NewEncoder(tc.params)
	tc.evaluator = ckks.NewEvaluator(tc.params, rlwe.EvaluationKey{})

	kgen := ckks.NewKeyGenerator(tc.params)

	// SecretKeys
	tc.sk0Shards = make([]*rlwe.SecretKey, parties)
	tc.sk1Shards = make([]*rlwe.SecretKey, parties)
	tc.sk0 = ckks.NewSecretKey(tc.params)
	tc.sk1 = ckks.NewSecretKey(tc.params)

	ringQP, levelQ, levelP := params.RingQP(), params.QCount()-1, params.PCount()-1
	for j := 0; j < parties; j++ {
		tc.sk0Shards[j] = kgen.GenSecretKey()
		tc.sk1Shards[j] = kgen.GenSecretKey()
		ringQP.AddLvl(levelQ, levelP, tc.sk0.Value, tc.sk0Shards[j].Value, tc.sk0.Value)
		ringQP.AddLvl(levelQ, levelP, tc.sk1.Value, tc.sk1Shards[j].Value, tc.sk1.Value)
	}

	// Publickeys
	tc.pk0 = kgen.GenPublicKey(tc.sk0)
	tc.pk1 = kgen.GenPublicKey(tc.sk1)

	tc.encryptorPk0 = ckks.NewEncryptor(tc.params, tc.pk0)
	tc.decryptorSk0 = ckks.NewDecryptor(tc.params, tc.sk0)
	tc.decryptorSk1 = ckks.NewDecryptor(tc.params, tc.sk1)

	return
}

func testPublicKeyGen(tc *testContext, t *testing.T) {

	decryptorSk0 := tc.decryptorSk0
	sk0Shards := tc.sk0Shards
	params := tc.params

	t.Run(testString("PublicKeyGen", parties, params), func(t *testing.T) {

		type Party struct {
			*CKGProtocol
			s  *rlwe.SecretKey
			s1 *drlwe.CKGShare
		}

		ckgParties := make([]*Party, parties)
		for i := 0; i < parties; i++ {
			p := new(Party)
			p.CKGProtocol = NewCKGProtocol(params)
			p.s = sk0Shards[i]
			p.s1 = p.AllocateShare()
			ckgParties[i] = p
		}
		P0 := ckgParties[0]

		crp := P0.SampleCRP(tc.crs)

		var _ drlwe.CollectivePublicKeyGenerator = P0.CKGProtocol

		// Each party creates a new CKGProtocol instance
		for i, p := range ckgParties {
			p.GenShare(p.s, crp, p.s1)
			if i > 0 {
				P0.AggregateShare(p.s1, P0.s1, P0.s1)
			}
		}

		pk := ckks.NewPublicKey(params)
		P0.GenPublicKey(P0.s1, crp, pk)

		// Verifies that decrypt((encryptp(collectiveSk, m), collectivePk) = m
		encryptorTest := ckks.NewEncryptor(params, pk)

		coeffs, _, ciphertext := newTestVectors(tc, encryptorTest, -1, 1)

		verifyTestVectors(tc, decryptorSk0, coeffs, ciphertext, t)
	})

}

func testRelinKeyGen(tc *testContext, t *testing.T) {

	encryptorPk0 := tc.encryptorPk0
	decryptorSk0 := tc.decryptorSk0
	sk0Shards := tc.sk0Shards
	params := tc.params

	t.Run(testString("RelinKeyGen", parties, params), func(t *testing.T) {

		type Party struct {
			*RKGProtocol
			ephSk  *rlwe.SecretKey
			sk     *rlwe.SecretKey
			share1 *drlwe.RKGShare
			share2 *drlwe.RKGShare
		}

		rkgParties := make([]*Party, parties)

		for i := range rkgParties {
			p := new(Party)
			p.RKGProtocol = NewRKGProtocol(params)
			p.sk = sk0Shards[i]
			p.ephSk, p.share1, p.share2 = p.AllocateShare()
			rkgParties[i] = p
		}

		P0 := rkgParties[0]

		crp := P0.SampleCRP(tc.crs)

		// Checks that ckks.RKGProtocol complies to the drlwe.RelinearizationKeyGenerator interface
		var _ drlwe.RelinearizationKeyGenerator = P0.RKGProtocol

		// ROUND 1
		for i, p := range rkgParties {
			p.GenShareRoundOne(p.sk, crp, p.ephSk, p.share1)
			if i > 0 {
				P0.AggregateShare(p.share1, P0.share1, P0.share1)
			}
		}

		//ROUND 2
		for i, p := range rkgParties {
			p.GenShareRoundTwo(p.ephSk, p.sk, P0.share1, p.share2)
			if i > 0 {
				P0.AggregateShare(p.share2, P0.share2, P0.share2)
			}
		}

		rlk := ckks.NewRelinearizationKey(params)
		P0.GenRelinearizationKey(P0.share1, P0.share2, rlk)

		coeffs, _, ciphertext := newTestVectors(tc, encryptorPk0, -1, 1)

		for i := range coeffs {
			coeffs[i] *= coeffs[i]
		}

		evaluator := tc.evaluator.WithKey(rlwe.EvaluationKey{Rlk: rlk, Rtks: nil})
		evaluator.MulRelin(ciphertext, ciphertext, ciphertext)

		if err := evaluator.Rescale(ciphertext, params.DefaultScale(), ciphertext); err != nil {
			t.Error(err)
		}

		require.Equal(t, ciphertext.Degree(), 1)

		verifyTestVectors(tc, decryptorSk0, coeffs, ciphertext, t)

	})

}

func testKeyswitching(tc *testContext, t *testing.T) {

	encryptorPk0 := tc.encryptorPk0
	decryptorSk1 := tc.decryptorSk1
	sk0Shards := tc.sk0Shards
	sk1Shards := tc.sk1Shards
	params := tc.params

	t.Run(testString("Keyswitching", parties, params), func(t *testing.T) {

		coeffs, _, ciphertextFullLevels := newTestVectors(tc, encryptorPk0, -1, 1)

		for _, dropped := range []int{0, ciphertextFullLevels.Level()} { // runs the test for full and level zero
			ciphertext := tc.evaluator.DropLevelNew(ciphertextFullLevels, dropped)

			t.Run(fmt.Sprintf("atLevel=%d", ciphertext.Level()), func(t *testing.T) {

				type Party struct {
					cks   *CKSProtocol
					s0    *rlwe.SecretKey
					s1    *rlwe.SecretKey
					share *drlwe.CKSShare
				}

				cksParties := make([]*Party, parties)
				for i := 0; i < parties; i++ {
					p := new(Party)
					p.cks = NewCKSProtocol(params, 3.2)
					p.s0 = sk0Shards[i]
					p.s1 = sk1Shards[i]
					p.share = p.cks.AllocateShare(ciphertext.Level())
					cksParties[i] = p
				}
				P0 := cksParties[0]

				// Checks that the protocol complies to the drlwe.KeySwitchingProtocol interface
				var _ drlwe.KeySwitchingProtocol = &P0.cks.CKSProtocol

				// Each party creates its CKSProtocol instance with tmp = si-si'
				for i, p := range cksParties {
					p.cks.GenShare(p.s0, p.s1, ciphertext.Value[1], p.share)
					if i > 0 {
						P0.cks.AggregateShare(p.share, P0.share, P0.share)
					}
				}

				ksCiphertext := ckks.NewCiphertext(params, 1, ciphertext.Level(), ciphertext.Scale()/2)

				P0.cks.KeySwitch(ciphertext, P0.share, ksCiphertext)

				verifyTestVectors(tc, decryptorSk1, coeffs, ksCiphertext, t)

				P0.cks.KeySwitch(ciphertext, P0.share, ciphertext)

				verifyTestVectors(tc, decryptorSk1, coeffs, ksCiphertext, t)

			})
		}
	})
}

func testPublicKeySwitching(tc *testContext, t *testing.T) {

	encryptorPk0 := tc.encryptorPk0
	decryptorSk1 := tc.decryptorSk1
	sk0Shards := tc.sk0Shards
	pk1 := tc.pk1
	params := tc.params

	t.Run(testString("PublicKeySwitching", parties, params), func(t *testing.T) {

		coeffs, _, ciphertextFullLevels := newTestVectors(tc, encryptorPk0, -1, 1)

		for _, dropped := range []int{0, ciphertextFullLevels.Level()} { // runs the test for full and level zero
			ciphertext := tc.evaluator.DropLevelNew(ciphertextFullLevels, dropped)

			t.Run(fmt.Sprintf("atLevel=%d", ciphertext.Level()), func(t *testing.T) {

				type Party struct {
					*PCKSProtocol
					s     *rlwe.SecretKey
					share *drlwe.PCKSShare
				}

				pcksParties := make([]*Party, parties)
				for i := 0; i < parties; i++ {
					p := new(Party)
					p.PCKSProtocol = NewPCKSProtocol(params, 3.2)
					p.s = sk0Shards[i]
					p.share = p.AllocateShare(ciphertext.Level())
					pcksParties[i] = p
				}
				P0 := pcksParties[0]

				// Checks that the protocol complies to the drlwe.KeySwitchingProtocol interface
				var _ drlwe.PublicKeySwitchingProtocol = &P0.PCKSProtocol.PCKSProtocol

				ciphertextSwitched := ckks.NewCiphertext(params, 1, ciphertext.Level(), ciphertext.Scale())

				for i, p := range pcksParties {
					p.GenShare(p.s, pk1, ciphertext.Value[1], p.share)
					if i > 0 {
						P0.AggregateShare(p.share, P0.share, P0.share)
					}
				}

				P0.KeySwitch(ciphertext, P0.share, ciphertextSwitched)

				verifyTestVectors(tc, decryptorSk1, coeffs, ciphertextSwitched, t)
			})
		}

	})
}

func testRotKeyGenConjugate(tc *testContext, t *testing.T) {

	encryptorPk0 := tc.encryptorPk0
	decryptorSk0 := tc.decryptorSk0
	sk0Shards := tc.sk0Shards
	params := tc.params

	t.Run(testString("RotKeyGenConjugate", parties, params), func(t *testing.T) {

		if tc.params.RingType() == ring.ConjugateInvariant {
			t.Skip("Conjugate not defined in Ring Conjugate Invariant")
		}

		type Party struct {
			*RTGProtocol
			s     *rlwe.SecretKey
			share *drlwe.RTGShare
		}

		pcksParties := make([]*Party, parties)
		for i := 0; i < parties; i++ {
			p := new(Party)
			p.RTGProtocol = NewRotKGProtocol(params)
			p.s = sk0Shards[i]
			p.share = p.AllocateShare()
			pcksParties[i] = p
		}
		P0 := pcksParties[0]

		// Checks that ckks.RTGProtocol complies to the drlwe.RotationKeyGenerator interface
		var _ drlwe.RotationKeyGenerator = P0.RTGProtocol

		crp := P0.SampleCRP(tc.crs)

		galEl := params.GaloisElementForRowRotation()
		rotKeySet := ckks.NewRotationKeySet(params, []uint64{galEl})

		for i, p := range pcksParties {
			p.GenShare(p.s, galEl, crp, p.share)
			if i > 0 {
				P0.AggregateShare(p.share, P0.share, P0.share)
			}
		}

		P0.GenRotationKey(P0.share, crp, rotKeySet.Keys[galEl])

		coeffs, _, ciphertext := newTestVectors(tc, encryptorPk0, -1, 1)

		evaluator := tc.evaluator.WithKey(rlwe.EvaluationKey{Rlk: nil, Rtks: rotKeySet})
		evaluator.Conjugate(ciphertext, ciphertext)

		coeffsWant := make([]complex128, params.Slots())

		for i := 0; i < params.Slots(); i++ {
			coeffsWant[i] = complex(real(coeffs[i]), -imag(coeffs[i]))
		}

		verifyTestVectors(tc, decryptorSk0, coeffsWant, ciphertext, t)

	})
}

func testRotKeyGenCols(tc *testContext, t *testing.T) {

	encryptorPk0 := tc.encryptorPk0
	decryptorSk0 := tc.decryptorSk0
	sk0Shards := tc.sk0Shards
	params := tc.params

	t.Run(testString("RotKeyGenCols", parties, params), func(t *testing.T) {

		type Party struct {
			*RTGProtocol
			s     *rlwe.SecretKey
			share *drlwe.RTGShare
		}

		pcksParties := make([]*Party, parties)
		for i := 0; i < parties; i++ {
			p := new(Party)
			p.RTGProtocol = NewRotKGProtocol(params)
			p.s = sk0Shards[i]
			p.share = p.AllocateShare()
			pcksParties[i] = p
		}

		P0 := pcksParties[0]

		crp := P0.SampleCRP(tc.crs)

		coeffs, _, ciphertext := newTestVectors(tc, encryptorPk0, -1, 1)

		receiver := ckks.NewCiphertext(params, ciphertext.Degree(), ciphertext.Level(), ciphertext.Scale())

		galEls := params.GaloisElementsForRowInnerSum()
		rotKeySet := ckks.NewRotationKeySet(params, galEls)

		for _, galEl := range galEls {
			for i, p := range pcksParties {
				p.GenShare(p.s, galEl, crp, p.share)
				if i > 0 {
					P0.AggregateShare(p.share, P0.share, P0.share)
				}
			}
			P0.GenRotationKey(P0.share, crp, rotKeySet.Keys[galEl])
		}

		evaluator := tc.evaluator.WithKey(rlwe.EvaluationKey{Rlk: nil, Rtks: rotKeySet})

		for k := 1; k < params.Slots(); k <<= 1 {
			evaluator.Rotate(ciphertext, int(k), receiver)

			coeffsWant := utils.RotateComplex128Slice(coeffs, int(k))

			verifyTestVectors(tc, decryptorSk0, coeffsWant, receiver, t)
		}
	})
}

func testE2SProtocol(tc *testContext, t *testing.T) {

	params := tc.params

	t.Run(testString("E2SProtocol", parties, params), func(t *testing.T) {

		var minLevel, logBound int
		var ok bool
		if minLevel, logBound, ok = GetMinimumLevelForBootstrapping(128, params.DefaultScale(), parties, params.Q()); ok != true || minLevel+1 > params.MaxLevel() {
			t.Skip("Not enough levels to ensure correcness and 128 security")
		}

		type Party struct {
			e2s            *E2SProtocol
			s2e            *S2EProtocol
			sk             *rlwe.SecretKey
			publicShareE2S *drlwe.CKSShare
			publicShareS2E *drlwe.CKSShare
			secretShare    *rlwe.AdditiveShareBigint
		}

		coeffs, _, ciphertext := newTestVectors(tc, tc.encryptorPk0, -1, 1)

		tc.evaluator.DropLevel(ciphertext, ciphertext.Level()-minLevel-1)

		params := tc.params
		P := make([]Party, parties)
		for i := range P {
			P[i].e2s = NewE2SProtocol(params, 3.2)
			P[i].s2e = NewS2EProtocol(params, 3.2)
			P[i].sk = tc.sk0Shards[i]
			P[i].publicShareE2S = P[i].e2s.AllocateShare(minLevel)
			P[i].publicShareS2E = P[i].s2e.AllocateShare(params.MaxLevel())
			P[i].secretShare = NewAdditiveShareBigint(params, params.LogSlots())
		}

		for i, p := range P {
			// Enc(-M_i)
			p.e2s.GenShare(p.sk, logBound, params.LogSlots(), ciphertext.Value[1], p.secretShare, p.publicShareE2S)

			if i > 0 {
				// Enc(sum(-M_i))
				p.e2s.AggregateShare(P[0].publicShareE2S, p.publicShareE2S, P[0].publicShareE2S)
			}
		}

		// sum(-M_i) + x
		P[0].e2s.GetShare(P[0].secretShare, P[0].publicShareE2S, params.LogSlots(), ciphertext, P[0].secretShare)

		// sum(-M_i) + x + sum(M_i) = x
		rec := NewAdditiveShareBigint(params, params.LogSlots())
		for _, p := range P {
			a := rec.Value
			b := p.secretShare.Value

			for i := range a {
				a[i].Add(a[i], b[i])
			}
		}

		pt := ckks.NewPlaintext(params, ciphertext.Level(), ciphertext.Scale())
		pt.Value.IsNTT = false
		tc.ringQ.SetCoefficientsBigintLvl(pt.Level(), rec.Value, pt.Value)

		verifyTestVectors(tc, nil, coeffs, pt, t)

		crp := P[0].s2e.SampleCRP(params.Parameters.MaxLevel(), tc.crs)

		for i, p := range P {
			p.s2e.GenShare(p.sk, crp, params.LogSlots(), p.secretShare, p.publicShareS2E)
			if i > 0 {
				p.s2e.AggregateShare(P[0].publicShareS2E, p.publicShareS2E, P[0].publicShareS2E)
			}
		}

		ctRec := ckks.NewCiphertext(params, 1, params.Parameters.MaxLevel(), ciphertext.Scale())
		P[0].s2e.GetEncryption(P[0].publicShareS2E, crp, ctRec)

		verifyTestVectors(tc, tc.decryptorSk0, coeffs, ctRec, t)

	})
}

func testRefresh(tc *testContext, t *testing.T) {

	encryptorPk0 := tc.encryptorPk0
	sk0Shards := tc.sk0Shards
	decryptorSk0 := tc.decryptorSk0
	params := tc.params

	t.Run(testString("Refresh", parties, params), func(t *testing.T) {

		var minLevel, logBound int
		var ok bool
		if minLevel, logBound, ok = GetMinimumLevelForBootstrapping(128, params.DefaultScale(), parties, params.Q()); ok != true || minLevel+1 > params.MaxLevel() {
			t.Skip("Not enough levels to ensure correcness and 128 security")
		}

		type Party struct {
			*RefreshProtocol
			s     *rlwe.SecretKey
			share *RefreshShare
		}

		levelIn := minLevel
		levelOut := params.MaxLevel()

		RefreshParties := make([]*Party, parties)
		for i := 0; i < parties; i++ {
			p := new(Party)
			if i == 0 {
				p.RefreshProtocol = NewRefreshProtocol(params, logBound, 3.2)
			} else {
				p.RefreshProtocol = RefreshParties[0].RefreshProtocol.ShallowCopy()
			}

			p.s = sk0Shards[i]
			p.share = p.AllocateShare(levelIn, levelOut)
			RefreshParties[i] = p
		}

		P0 := RefreshParties[0]

		for _, scale := range []float64{params.DefaultScale(), params.DefaultScale() * 128} {
			t.Run(fmt.Sprintf("atScale=%f", scale), func(t *testing.T) {
				coeffs, _, ciphertext := newTestVectorsAtScale(tc, encryptorPk0, -1, 1, scale)

				// Brings ciphertext to minLevel + 1
				tc.evaluator.DropLevel(ciphertext, ciphertext.Level()-minLevel-1)

				crp := P0.SampleCRP(levelOut, tc.crs)

				for i, p := range RefreshParties {

					p.GenShare(p.s, logBound, params.LogSlots(), ciphertext.Value[1], ciphertext.Scale(), crp, p.share)

					if i > 0 {
						P0.AggregateShare(p.share, P0.share, P0.share)
					}
				}

				P0.Finalize(ciphertext, params.LogSlots(), crp, P0.share, ciphertext)

				verifyTestVectors(tc, decryptorSk0, coeffs, ciphertext, t)
			})
		}

	})
}

func testRefreshAndTransform(tc *testContext, t *testing.T) {

	encryptorPk0 := tc.encryptorPk0
	sk0Shards := tc.sk0Shards
	params := tc.params
	decryptorSk0 := tc.decryptorSk0

	t.Run(testString("RefreshAndTransform", parties, params), func(t *testing.T) {

		var minLevel, logBound int
		var ok bool
		if minLevel, logBound, ok = GetMinimumLevelForBootstrapping(128, params.DefaultScale(), parties, params.Q()); ok != true || minLevel+1 > params.MaxLevel() {
			t.Skip("Not enough levels to ensure correcness and 128 security")
		}

		type Party struct {
			*MaskedTransformProtocol
			s     *rlwe.SecretKey
			share *MaskedTransformShare
		}

		coeffs, _, ciphertext := newTestVectors(tc, encryptorPk0, -1, 1)

		// Drops the ciphertext to the minimum level that ensures correctness and 128-bit security
		tc.evaluator.DropLevel(ciphertext, ciphertext.Level()-minLevel-1)

		levelIn := minLevel
		levelOut := params.MaxLevel()

		RefreshParties := make([]*Party, parties)
		var err error
		for i := 0; i < parties; i++ {
			p := new(Party)

			if i == 0 {
				if p.MaskedTransformProtocol, err = NewMaskedTransformProtocol(params, params, logBound, 3.2); err != nil {
					t.Log(err)
					t.Fail()
				}
			} else {
				p.MaskedTransformProtocol = RefreshParties[0].MaskedTransformProtocol.ShallowCopy()
			}

			p.s = sk0Shards[i]
			p.share = p.AllocateShare(levelIn, levelOut)
			RefreshParties[i] = p
		}

		P0 := RefreshParties[0]
		crp := P0.SampleCRP(levelOut, tc.crs)

		transform := &MaskedTransformFunc{
			Decode: true,
			Func: func(coeffs []*ring.Complex) {
				for i := range coeffs {
					coeffs[i][0].Mul(coeffs[i][0], ring.NewFloat(0.9238795325112867, logBound))
					coeffs[i][1].Mul(coeffs[i][1], ring.NewFloat(0.7071067811865476, logBound))
				}
			},
			Encode: true,
		}

		for i, p := range RefreshParties {
			p.GenShare(p.s, p.s, logBound, params.LogSlots(), ciphertext.Value[1], ciphertext.Scale(), crp, transform, p.share)

			if i > 0 {
				P0.AggregateShare(p.share, P0.share, P0.share)
			}
		}

		P0.Transform(ciphertext, tc.params.LogSlots(), transform, crp, P0.share, ciphertext)

		for i := range coeffs {
			coeffs[i] = complex(real(coeffs[i])*0.9238795325112867, imag(coeffs[i])*0.7071067811865476)
		}

		verifyTestVectors(tc, decryptorSk0, coeffs, ciphertext, t)
	})
}

func testRefreshAndTransformSwitchParams(tc *testContext, t *testing.T) {

	var err error

	encryptorPk0 := tc.encryptorPk0
	sk0Shards := tc.sk0Shards
	params := tc.params

	t.Run(testString("RefreshAndTransformAndSwitchParams", parties, params), func(t *testing.T) {

		var minLevel, logBound int
		var ok bool
		if minLevel, logBound, ok = GetMinimumLevelForBootstrapping(128, params.DefaultScale(), parties, params.Q()); ok != true || minLevel+1 > params.MaxLevel() {
			t.Skip("Not enough levels to ensure correcness and 128 security")
		}

		type Party struct {
			*MaskedTransformProtocol
			sIn   *rlwe.SecretKey
			sOut  *rlwe.SecretKey
			share *MaskedTransformShare
		}

		coeffs, _, ciphertext := newTestVectors(tc, encryptorPk0, -1, 1)

		// Drops the ciphertext to the minimum level that ensures correctness and 128-bit security
		tc.evaluator.DropLevel(ciphertext, ciphertext.Level()-minLevel-1)

		levelIn := minLevel

		// Target parameters
		var paramsOut ckks.Parameters
		paramsOut, err = ckks.NewParametersFromLiteral(ckks.ParametersLiteral{
			LogN:         params.LogN() + 1,
			LogQ:         []int{54, 49, 49, 49, 49, 49, 49},
			LogP:         []int{52, 52},
			RingType:     params.RingType(),
			LogSlots:     params.MaxLogSlots() + 1,
			DefaultScale: 1 << 49,
		})

		require.Nil(t, err)

		levelOut := paramsOut.MaxLevel()

		RefreshParties := make([]*Party, parties)

		kgenParamsOut := rlwe.NewKeyGenerator(paramsOut.Parameters)
		skIdealOut := rlwe.NewSecretKey(paramsOut.Parameters)
		for i := 0; i < parties; i++ {
			p := new(Party)

			if i == 0 {
				if p.MaskedTransformProtocol, err = NewMaskedTransformProtocol(params, paramsOut, logBound, 3.2); err != nil {
					t.Log(err)
					t.Fail()
				}
			} else {
				p.MaskedTransformProtocol = RefreshParties[0].MaskedTransformProtocol.ShallowCopy()
			}

			p.sIn = sk0Shards[i]

			p.sOut = kgenParamsOut.GenSecretKey() // New shared secret key in target parameters
			paramsOut.RingQ().Add(skIdealOut.Value.Q, p.sOut.Value.Q, skIdealOut.Value.Q)

			p.share = p.AllocateShare(levelIn, levelOut)
			RefreshParties[i] = p
		}

		P0 := RefreshParties[0]
		crp := P0.SampleCRP(levelOut, tc.crs)

		transform := &MaskedTransformFunc{
			Decode: true,
			Func: func(coeffs []*ring.Complex) {
				for i := range coeffs {
					coeffs[i][0].Mul(coeffs[i][0], ring.NewFloat(0.9238795325112867, logBound))
					coeffs[i][1].Mul(coeffs[i][1], ring.NewFloat(0.7071067811865476, logBound))
				}
			},
			Encode: true,
		}

		for i, p := range RefreshParties {
			p.GenShare(p.sIn, p.sOut, logBound, params.LogSlots(), ciphertext.Value[1], ciphertext.Scale(), crp, transform, p.share)

			if i > 0 {
				P0.AggregateShare(p.share, P0.share, P0.share)
			}
		}

		P0.Transform(ciphertext, tc.params.LogSlots(), transform, crp, P0.share, ciphertext)

		for i := range coeffs {
			coeffs[i] = complex(real(coeffs[i])*0.9238795325112867, imag(coeffs[i])*0.7071067811865476)
		}

		precStats := ckks.GetPrecisionStats(paramsOut, ckks.NewEncoder(paramsOut), nil, coeffs, ckks.NewDecryptor(paramsOut, skIdealOut).DecryptNew(ciphertext), params.LogSlots(), 0)

		if *printPrecisionStats {
			t.Log(precStats.String())
		}

		require.GreaterOrEqual(t, precStats.MeanPrecision.Real, minPrec)
		require.GreaterOrEqual(t, precStats.MeanPrecision.Imag, minPrec)
	})
}

func testMarshalling(tc *testContext, t *testing.T) {
	params := tc.params

	t.Run(testString("Marshalling/Refresh", parties, params), func(t *testing.T) {

		var minLevel, logBound int
		var ok bool
		if minLevel, logBound, ok = GetMinimumLevelForBootstrapping(128, params.DefaultScale(), parties, params.Q()); ok != true {
			t.Skip("Not enough levels to ensure correcness and 128 security")
		}

		ciphertext := ckks.NewCiphertext(params, 1, minLevel, params.DefaultScale())
		tc.uniformSampler.Read(ciphertext.Value[0])
		tc.uniformSampler.Read(ciphertext.Value[1])

		// Testing refresh shares
		refreshproto := NewRefreshProtocol(tc.params, logBound, 3.2)
		refreshshare := refreshproto.AllocateShare(ciphertext.Level(), params.MaxLevel())

		crp := refreshproto.SampleCRP(params.MaxLevel(), tc.crs)

		refreshproto.GenShare(tc.sk0, logBound, params.LogSlots(), ciphertext.Value[1], ciphertext.Scale(), crp, refreshshare)

		data, err := refreshshare.MarshalBinary()

		if err != nil {
			t.Fatal("Could not marshal RefreshShare", err)
		}

		resRefreshShare := new(MaskedTransformShare)
		err = resRefreshShare.UnmarshalBinary(data)

		if err != nil {
			t.Fatal("Could not unmarshal RefreshShare", err)
		}

		for i, r := range refreshshare.e2sShare.Value.Coeffs {
			if !utils.EqualSliceUint64(resRefreshShare.e2sShare.Value.Coeffs[i], r) {
				t.Fatal("Result of marshalling not the same as original : RefreshShare")
			}

		}
		for i, r := range refreshshare.s2eShare.Value.Coeffs {
			if !utils.EqualSliceUint64(resRefreshShare.s2eShare.Value.Coeffs[i], r) {
				t.Fatal("Result of marshalling not the same as original : RefreshShare")
			}

		}
	})
}

func newTestVectors(testContext *testContext, encryptor ckks.Encryptor, a, b complex128) (values []complex128, plaintext *ckks.Plaintext, ciphertext *ckks.Ciphertext) {
	return newTestVectorsAtScale(testContext, encryptor, a, b, testContext.params.DefaultScale())
}

func newTestVectorsAtScale(testContext *testContext, encryptor ckks.Encryptor, a, b complex128, scale float64) (values []complex128, plaintext *ckks.Plaintext, ciphertext *ckks.Ciphertext) {

	params := testContext.params

	logSlots := params.LogSlots()

	values = make([]complex128, 1<<logSlots)

	for i := 0; i < 1<<logSlots; i++ {
		values[i] = complex(utils.RandFloat64(real(a), real(b)), utils.RandFloat64(imag(a), imag(b)))
	}

	plaintext = testContext.encoder.EncodeNew(values, params.MaxLevel(), scale, params.LogSlots())

	if encryptor != nil {
		ciphertext = encryptor.EncryptNew(plaintext)
	}

	return values, plaintext, ciphertext
}

func verifyTestVectors(tc *testContext, decryptor ckks.Decryptor, valuesWant []complex128, element interface{}, t *testing.T) {

	precStats := ckks.GetPrecisionStats(tc.params, tc.encoder, decryptor, valuesWant, element, tc.params.LogSlots(), 0)

	if *printPrecisionStats {
		t.Log(precStats.String())
	}

	require.GreaterOrEqual(t, precStats.MeanPrecision.Real, minPrec)
	require.GreaterOrEqual(t, precStats.MeanPrecision.Imag, minPrec)
}
