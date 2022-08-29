package dbgv

import (
	"encoding/json"
	"flag"
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tuneinsight/lattigo/v3/bgv"
	"github.com/tuneinsight/lattigo/v3/drlwe"
	"github.com/tuneinsight/lattigo/v3/ring"
	"github.com/tuneinsight/lattigo/v3/rlwe"
	"github.com/tuneinsight/lattigo/v3/utils"
)

var flagLongTest = flag.Bool("long", false, "run the long test suite (all parameters). Overrides -short and requires -timeout=0.")
var flagParamString = flag.String("params", "", "specify the test cryptographic parameters as a JSON string. Overrides -short and -long.")
var parties int = 3

func testString(opname string, parties int, params bgv.Parameters) string {
	return fmt.Sprintf("%s/LogN=%d/logQ=%d/parties=%d", opname, params.LogN(), params.LogQP(), parties)
}

type testContext struct {
	params bgv.Parameters

	// Polynomial degree
	n int

	// Polynomial contexts
	ringT *ring.Ring
	ringQ *ring.Ring
	ringP *ring.Ring

	encoder bgv.Encoder

	sk0Shards []*rlwe.SecretKey
	sk0       *rlwe.SecretKey

	sk1       *rlwe.SecretKey
	sk1Shards []*rlwe.SecretKey

	pk0 *rlwe.PublicKey
	pk1 *rlwe.PublicKey

	encryptorPk0 bgv.Encryptor
	decryptorSk0 bgv.Decryptor
	decryptorSk1 bgv.Decryptor
	evaluator    bgv.Evaluator

	crs            drlwe.CRS
	uniformSampler *ring.UniformSampler
}

func TestDBGV(t *testing.T) {

	var err error

	defaultParams := bgv.DefaultParams[:] // the default test runs for ring degree N=2^12, 2^13, 2^14, 2^15
	if testing.Short() {
		defaultParams = bgv.DefaultParams[:2] // the short test suite runs for ring degree N=2^12, 2^13
	}
	if *flagLongTest {
		defaultParams = append(defaultParams, bgv.DefaultPostQuantumParams...) // the long test suite runs for all default parameters
	}
	if *flagParamString != "" {
		var jsonParams bgv.ParametersLiteral
		if err = json.Unmarshal([]byte(*flagParamString), &jsonParams); err != nil {
			t.Fatal(err)
		}
		defaultParams = []bgv.ParametersLiteral{jsonParams} // the custom test suite reads the parameters from the -params flag
	}

	for _, p := range defaultParams {

		var params bgv.Parameters
		if params, err = bgv.NewParametersFromLiteral(p); err != nil {
			t.Fatal(err)
		}

		var tc *testContext
		if tc, err = gentestContext(params); err != nil {
			t.Fatal(err)
		}
		for _, testSet := range []func(tc *testContext, t *testing.T){
			testKeyswitching,
			testPublicKeySwitching,
			testEncToShares,
			testRefresh,
			testRefreshAndPermutation,
			testMarshalling,
		} {
			testSet(tc, t)
			runtime.GC()
		}
	}
}

func gentestContext(params bgv.Parameters) (tc *testContext, err error) {

	tc = new(testContext)

	tc.params = params

	tc.n = params.N()

	tc.ringT = params.RingT()
	tc.ringQ = params.RingQ()
	tc.ringP = params.RingP()

	prng, _ := utils.NewKeyedPRNG([]byte{'t', 'e', 's', 't'})
	tc.crs = prng
	tc.uniformSampler = ring.NewUniformSampler(prng, params.RingQ())

	tc.encoder = bgv.NewEncoder(tc.params)
	tc.evaluator = bgv.NewEvaluator(tc.params, rlwe.EvaluationKey{})

	kgen := bgv.NewKeyGenerator(tc.params)

	// SecretKeys
	tc.sk0Shards = make([]*rlwe.SecretKey, parties)
	tc.sk1Shards = make([]*rlwe.SecretKey, parties)

	tc.sk0 = bgv.NewSecretKey(tc.params)
	tc.sk1 = bgv.NewSecretKey(tc.params)

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

	tc.encryptorPk0 = bgv.NewEncryptor(tc.params, tc.pk0)
	tc.decryptorSk0 = bgv.NewDecryptor(tc.params, tc.sk0)
	tc.decryptorSk1 = bgv.NewDecryptor(tc.params, tc.sk1)

	return
}

func testKeyswitching(tc *testContext, t *testing.T) {

	sk0Shards := tc.sk0Shards
	sk1Shards := tc.sk1Shards
	encryptorPk0 := tc.encryptorPk0
	decryptorSk1 := tc.decryptorSk1

	t.Run(testString("KeySwitching", parties, tc.params), func(t *testing.T) {

		coeffs, _, ciphertext := newTestVectors(tc, encryptorPk0, t)

		type Party struct {
			cks   *CKSProtocol
			s0    *rlwe.SecretKey
			s1    *rlwe.SecretKey
			share *drlwe.CKSShare
		}

		cksParties := make([]*Party, parties)
		for i := 0; i < parties; i++ {
			p := new(Party)
			p.cks = NewCKSProtocol(tc.params, 6.36)
			p.s0 = sk0Shards[i]
			p.s1 = sk1Shards[i]
			p.share = p.cks.AllocateShare(ciphertext.Level())
			cksParties[i] = p
		}
		P0 := cksParties[0]

		// Checks that the protocol complies to the drlwe.PublicKeySwitchingProtocol interface
		var _ drlwe.KeySwitchingProtocol = &P0.cks.CKSProtocol

		// Each party creates its CKSProtocol instance with tmp = si-si'
		for i, p := range cksParties {
			p.cks.GenShare(p.s0, p.s1, ciphertext.Value[1], p.share)
			if i > 0 {
				P0.cks.AggregateShare(p.share, P0.share, P0.share)
			}
		}

		ksCiphertext := bgv.NewCiphertext(tc.params, 1, ciphertext.Level(), 1)
		P0.cks.KeySwitch(ciphertext, P0.share, ksCiphertext)

		verifyTestVectors(tc, decryptorSk1, coeffs, ksCiphertext, t)

		P0.cks.KeySwitch(ciphertext, P0.share, ciphertext)

		verifyTestVectors(tc, decryptorSk1, coeffs, ciphertext, t)

	})
}

func testPublicKeySwitching(tc *testContext, t *testing.T) {

	sk0Shards := tc.sk0Shards
	pk1 := tc.pk1
	encryptorPk0 := tc.encryptorPk0
	decryptorSk1 := tc.decryptorSk1

	t.Run(testString("PublicKeySwitching", parties, tc.params), func(t *testing.T) {

		type Party struct {
			*PCKSProtocol
			s     *rlwe.SecretKey
			share *drlwe.PCKSShare
		}

		coeffs, _, ciphertext := newTestVectors(tc, encryptorPk0, t)

		pcksParties := make([]*Party, parties)
		for i := 0; i < parties; i++ {
			p := new(Party)
			p.PCKSProtocol = NewPCKSProtocol(tc.params, 6.36)
			p.s = sk0Shards[i]
			p.share = p.AllocateShare(ciphertext.Level())
			pcksParties[i] = p
		}
		P0 := pcksParties[0]

		// Checks that the protocol complies to the drlwe.PublicKeySwitchingProtocol interface
		var _ drlwe.PublicKeySwitchingProtocol = &P0.PCKSProtocol.PCKSProtocol

		ciphertextSwitched := bgv.NewCiphertext(tc.params, 1, ciphertext.Level(), ciphertext.Scale())

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

func testEncToShares(tc *testContext, t *testing.T) {

	coeffs, _, ciphertext := newTestVectors(tc, tc.encryptorPk0, t)

	type Party struct {
		e2s         *E2SProtocol
		s2e         *S2EProtocol
		sk          *rlwe.SecretKey
		publicShare *drlwe.CKSShare
		secretShare *rlwe.AdditiveShare
	}

	params := tc.params
	P := make([]Party, parties)

	for i := range P {
		if i == 0 {
			P[i].e2s = NewE2SProtocol(params, 3.2)
			P[i].s2e = NewS2EProtocol(params, 3.2)
		} else {
			P[i].e2s = P[0].e2s.ShallowCopy()
			P[i].s2e = P[0].s2e.ShallowCopy()
		}

		P[i].sk = tc.sk0Shards[i]
		P[i].publicShare = P[i].e2s.AllocateShare(ciphertext.Level())
		P[i].secretShare = rlwe.NewAdditiveShare(params.Parameters)
	}

	// The E2S protocol is run in all tests, as a setup to the S2E test.
	for i, p := range P {
		p.e2s.GenShare(p.sk, ciphertext.Value[1], p.secretShare, p.publicShare)
		if i > 0 {
			p.e2s.AggregateShare(P[0].publicShare, p.publicShare, P[0].publicShare)
		}
	}

	P[0].e2s.GetShare(P[0].secretShare, P[0].publicShare, ciphertext, P[0].secretShare)

	t.Run(testString("E2SProtocol", parties, tc.params), func(t *testing.T) {

		rec := rlwe.NewAdditiveShare(params.Parameters)
		for _, p := range P {
			tc.ringT.Add(&rec.Value, &p.secretShare.Value, &rec.Value)
		}

		ptRt := tc.params.RingT().NewPoly()
		ptRt.IsNTT = true
		ptRt.Copy(&rec.Value)
		values := make([]uint64, len(coeffs))

		tc.encoder.DecodeRingT(ptRt, ciphertext.Scale(), values)
		assert.True(t, utils.EqualSliceUint64(coeffs, values))
	})

	crp := P[0].e2s.SampleCRP(params.MaxLevel(), tc.crs)

	t.Run(testString("S2EProtocol", parties, tc.params), func(t *testing.T) {

		for i, p := range P {
			p.s2e.GenShare(p.sk, crp, p.secretShare, p.publicShare)
			if i > 0 {
				p.s2e.AggregateShare(P[0].publicShare, p.publicShare, P[0].publicShare)
			}
		}

		ctRec := bgv.NewCiphertext(tc.params, 1, tc.params.MaxLevel(), ciphertext.Scale())
		P[0].s2e.GetEncryption(P[0].publicShare, crp, ctRec)

		verifyTestVectors(tc, tc.decryptorSk0, coeffs, ctRec, t)
	})
}

func testRefresh(tc *testContext, t *testing.T) {

	encryptorPk0 := tc.encryptorPk0
	sk0Shards := tc.sk0Shards
	encoder := tc.encoder
	decryptorSk0 := tc.decryptorSk0

	minLevel := 0
	maxLevel := tc.params.MaxLevel()

	t.Run(testString("Refresh", parties, tc.params), func(t *testing.T) {

		type Party struct {
			*RefreshProtocol
			s     *rlwe.SecretKey
			share *RefreshShare
		}

		RefreshParties := make([]*Party, parties)
		for i := 0; i < parties; i++ {
			p := new(Party)
			if i == 0 {
				p.RefreshProtocol = NewRefreshProtocol(tc.params, 3.2)
			} else {
				p.RefreshProtocol = RefreshParties[0].RefreshProtocol.ShallowCopy()
			}

			p.s = sk0Shards[i]
			p.share = p.AllocateShare(minLevel, maxLevel)
			RefreshParties[i] = p
		}

		P0 := RefreshParties[0]

		crp := P0.SampleCRP(maxLevel, tc.crs)

		coeffs, _, ciphertext := newTestVectors(tc, encryptorPk0, t)
		ciphertext.Resize(ciphertext.Degree(), minLevel)

		for i, p := range RefreshParties {
			p.GenShare(p.s, ciphertext.Value[1], ciphertext.Scale(), crp, p.share)
			if i > 0 {
				P0.AggregateShare(p.share, P0.share, P0.share)
			}

		}

		P0.Finalize(ciphertext, crp, P0.share, ciphertext)

		//Decrypts and compare
		require.True(t, ciphertext.Level() == maxLevel)
		require.True(t, utils.EqualSliceUint64(coeffs, encoder.DecodeUintNew(decryptorSk0.DecryptNew(ciphertext))))
	})
}

func testRefreshAndPermutation(tc *testContext, t *testing.T) {

	encryptorPk0 := tc.encryptorPk0
	sk0Shards := tc.sk0Shards
	encoder := tc.encoder
	decryptorSk0 := tc.decryptorSk0

	minLevel := 0
	maxLevel := tc.params.MaxLevel()

	t.Run(testString("RefreshAndPermutation", parties, tc.params), func(t *testing.T) {

		type Party struct {
			*MaskedTransformProtocol
			s     *rlwe.SecretKey
			share *MaskedTransformShare
		}

		RefreshParties := make([]*Party, parties)
		for i := 0; i < parties; i++ {
			p := new(Party)
			if i == 0 {
				p.MaskedTransformProtocol = NewMaskedTransformProtocol(tc.params, 3.2)
			} else {
				p.MaskedTransformProtocol = NewMaskedTransformProtocol(tc.params, 3.2)
			}

			p.s = sk0Shards[i]
			p.share = p.AllocateShare(minLevel, maxLevel)
			RefreshParties[i] = p
		}

		P0 := RefreshParties[0]

		crp := P0.SampleCRP(maxLevel, tc.crs)

		coeffs, _, ciphertext := newTestVectors(tc, encryptorPk0, t)
		ciphertext.Resize(ciphertext.Degree(), minLevel)

		permutation := make([]uint64, len(coeffs))
		N := uint64(tc.params.N())
		prng, _ := utils.NewPRNG()
		for i := range permutation {
			permutation[i] = ring.RandUniform(prng, N, N-1)
		}

		permute := func(coeffs []uint64) {
			coeffsPerm := make([]uint64, len(coeffs))
			for i := range coeffs {
				coeffsPerm[i] = coeffs[permutation[i]]
			}
			copy(coeffs, coeffsPerm)
		}

		maskedTransform := &MaskedTransformFunc{
			Decode: true,
			Func:   permute,
			Encode: true,
		}

		for i, p := range RefreshParties {
			p.GenShare(p.s, ciphertext.Value[1], ciphertext.Scale(), crp, maskedTransform, p.share)
			if i > 0 {
				P0.AggregateShare(P0.share, p.share, P0.share)
			}
		}

		P0.Transform(ciphertext, maskedTransform, crp, P0.share, ciphertext)

		coeffsPermute := make([]uint64, len(coeffs))
		for i := range coeffsPermute {
			coeffsPermute[i] = coeffs[permutation[i]]
		}

		coeffsHave := encoder.DecodeUintNew(decryptorSk0.DecryptNew(ciphertext))

		//Decrypts and compares
		require.True(t, ciphertext.Level() == maxLevel)
		require.True(t, utils.EqualSliceUint64(coeffsPermute, coeffsHave))
	})
}

func newTestVectors(tc *testContext, encryptor bgv.Encryptor, t *testing.T) (coeffs []uint64, plaintext *bgv.Plaintext, ciphertext *bgv.Ciphertext) {

	prng, _ := utils.NewPRNG()
	uniformSampler := ring.NewUniformSampler(prng, tc.ringT)
	coeffsPol := uniformSampler.ReadNew()

	for i := range coeffsPol.Coeffs[0] {
		coeffsPol.Coeffs[0][i] = uint64(1)
	}

	plaintext = bgv.NewPlaintext(tc.params, tc.params.MaxLevel(), 2)
	tc.encoder.Encode(coeffsPol.Coeffs[0], plaintext)
	ciphertext = encryptor.EncryptNew(plaintext)
	return coeffsPol.Coeffs[0], plaintext, ciphertext
}

func verifyTestVectors(tc *testContext, decryptor bgv.Decryptor, coeffs []uint64, ciphertext *bgv.Ciphertext, t *testing.T) {
	require.True(t, utils.EqualSliceUint64(coeffs, tc.encoder.DecodeUintNew(decryptor.DecryptNew(ciphertext))))
}

func testMarshalling(tc *testContext, t *testing.T) {
	ciphertext := bgv.NewCiphertext(tc.params, 1, tc.params.MaxLevel(), 1)
	tc.uniformSampler.Read(ciphertext.Value[0])
	tc.uniformSampler.Read(ciphertext.Value[1])

	minLevel := 0
	maxLevel := tc.params.MaxLevel()

	t.Run(testString("MarshallingRefresh", parties, tc.params), func(t *testing.T) {

		// Testing refresh shares
		refreshproto := NewRefreshProtocol(tc.params, 3.2)
		refreshshare := refreshproto.AllocateShare(minLevel, maxLevel)

		crp := refreshproto.SampleCRP(maxLevel, tc.crs)

		refreshproto.GenShare(tc.sk0, ciphertext.Value[1], ciphertext.Scale(), crp, refreshshare)

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
