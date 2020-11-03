package ring

import (
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"math/rand"
	"testing"
	"time"

	"github.com/ldsec/lattigo/v2/utils"

	"github.com/stretchr/testify/require"
)

var T = uint64(0x3ee0001)
var DefaultSigma = float64(3.2)
var DefaultBound = uint64(6 * DefaultSigma)

func testString(opname string, ringQ *Ring) string {
	return fmt.Sprintf("%sN=%d/limbs=%d", opname, ringQ.N, len(ringQ.Modulus))
}

type testParams struct {
	ringQ           *Ring
	ringP           *Ring
	prng            utils.PRNG
	uniformSamplerQ *UniformSampler
	uniformSamplerP *UniformSampler
}

func genTestParams(defaultParams *Parameters) (testContext *testParams, err error) {

	testContext = new(testParams)

	if testContext.ringQ, err = NewRing(1<<defaultParams.logN, defaultParams.qi); err != nil {
		return nil, err
	}
	if testContext.ringP, err = NewRing(1<<defaultParams.logN, defaultParams.pi); err != nil {
		return nil, err
	}
	if testContext.prng, err = utils.NewPRNG(); err != nil {
		return nil, err
	}
	testContext.uniformSamplerQ = NewUniformSampler(testContext.prng, testContext.ringQ)
	testContext.uniformSamplerP = NewUniformSampler(testContext.prng, testContext.ringP)
	return
}

func TestRing(t *testing.T) {

	rand.Seed(time.Now().UnixNano())
	var err error
	var defaultParams []*Parameters

	if testing.Short() {
		defaultParams = DefaultParams[:3]
	} else {
		defaultParams = DefaultParams
	}

	var testContext = new(testParams)

	for _, defaultParam := range defaultParams {

		if testContext, err = genTestParams(defaultParam); err != nil {
			panic(err)
		}

		testPRNG(testContext, t)
		testGenerateNTTPrimes(testContext, t)
		testImportExportPolyString(testContext, t)
		testDivFloorByLastModulusMany(testContext, t)
		testDivRoundByLastModulusMany(testContext, t)
		testMarshalBinary(testContext, t)
		testUniformSampler(testContext, t)
		testGaussianSampler(testContext, t)
		testTernarySampler(testContext, t)
		testGaloisShift(testContext, t)
		testModularReduction(testContext, t)
		testMulScalarBigint(testContext, t)
		testMulPoly(testContext, t)
		testExtendBasis(testContext, t)
		testScaling(testContext, t)
		testMultByMonomial(testContext, t)
	}
}

func testPRNG(testContext *testParams, t *testing.T) {

	sum := make([]byte, testContext.ringQ.N)
	t.Run(testString("PRNG/", testContext.ringQ), func(t *testing.T) {
		prng1, err := utils.NewKeyedPRNG(nil)
		if err != nil {
			panic(err)
		}
		prng2, err := utils.NewKeyedPRNG(nil)
		if err != nil {
			panic(err)
		}

		prng1.SetClock(sum, 256)
		prng2.SetClock(sum, 256)

		crsGenerator1 := NewUniformSampler(prng1, testContext.ringQ)
		crsGenerator2 := NewUniformSampler(prng2, testContext.ringQ)

		p0 := crsGenerator1.ReadNew()
		p1 := crsGenerator2.ReadNew()

		require.True(t, testContext.ringQ.Equal(p0, p1))
	})

}

func testGenerateNTTPrimes(testContext *testParams, t *testing.T) {

	t.Run(testString("GenerateNTTPrimes/", testContext.ringQ), func(t *testing.T) {

		primes := GenerateNTTPrimes(55, uint64(bits.Len64(testContext.ringQ.N)-1), uint64(len(testContext.ringQ.Modulus)))

		for _, q := range primes {
			require.Equal(t, q&((testContext.ringQ.N<<1)-1), uint64(1))
			require.True(t, IsPrime(q), q)
		}
	})
}

func testImportExportPolyString(testContext *testParams, t *testing.T) {

	t.Run(testString("ImportExportPolyString/", testContext.ringQ), func(t *testing.T) {

		p0 := testContext.uniformSamplerQ.ReadNew()
		p1 := testContext.ringQ.NewPoly()

		testContext.ringQ.SetCoefficientsString(testContext.ringQ.PolyToString(p0), p1)

		require.True(t, testContext.ringQ.Equal(p0, p1))
	})
}

func testDivFloorByLastModulusMany(testContext *testParams, t *testing.T) {

	t.Run(testString("DivFloorByLastModulusMany/", testContext.ringQ), func(t *testing.T) {

		coeffs := make([]*big.Int, testContext.ringQ.N)
		for i := uint64(0); i < testContext.ringQ.N; i++ {
			coeffs[i] = RandInt(testContext.ringQ.ModulusBigint)
			coeffs[i].Quo(coeffs[i], NewUint(10))
		}

		nbRescals := len(testContext.ringQ.Modulus) - 1

		coeffsWant := make([]*big.Int, testContext.ringQ.N)
		for i := range coeffs {
			coeffsWant[i] = new(big.Int).Set(coeffs[i])
			for j := 0; j < nbRescals; j++ {
				coeffsWant[i].Quo(coeffsWant[i], NewUint(testContext.ringQ.Modulus[len(testContext.ringQ.Modulus)-1-j]))
			}
		}

		polTest := testContext.ringQ.NewPoly()
		polWant := testContext.ringQ.NewPoly()

		testContext.ringQ.SetCoefficientsBigint(coeffs, polTest)
		testContext.ringQ.SetCoefficientsBigint(coeffsWant, polWant)

		testContext.ringQ.DivFloorByLastModulusMany(polTest, uint64(nbRescals))
		for i := uint64(0); i < testContext.ringQ.N; i++ {
			for j := 0; j < len(testContext.ringQ.Modulus)-nbRescals; j++ {
				require.Equalf(t, polWant.Coeffs[j][i], polTest.Coeffs[j][i], "coeff %v Qi%v = %s", i, j, coeffs[i].String())
			}
		}
	})
}

func testDivRoundByLastModulusMany(testContext *testParams, t *testing.T) {

	t.Run(testString("DivRoundByLastModulusMany/", testContext.ringQ), func(t *testing.T) {

		coeffs := make([]*big.Int, testContext.ringQ.N)
		for i := uint64(0); i < testContext.ringQ.N; i++ {
			coeffs[i] = RandInt(testContext.ringQ.ModulusBigint)
			coeffs[i].Quo(coeffs[i], NewUint(10))
		}

		nbRescals := len(testContext.ringQ.Modulus) - 1

		coeffsWant := make([]*big.Int, testContext.ringQ.N)
		for i := range coeffs {
			coeffsWant[i] = new(big.Int).Set(coeffs[i])
			for j := 0; j < nbRescals; j++ {
				DivRound(coeffsWant[i], NewUint(testContext.ringQ.Modulus[len(testContext.ringQ.Modulus)-1-j]), coeffsWant[i])
			}
		}

		polTest := testContext.ringQ.NewPoly()
		polWant := testContext.ringQ.NewPoly()

		testContext.ringQ.SetCoefficientsBigint(coeffs, polTest)
		testContext.ringQ.SetCoefficientsBigint(coeffsWant, polWant)

		testContext.ringQ.DivRoundByLastModulusMany(polTest, uint64(nbRescals))
		for i := uint64(0); i < testContext.ringQ.N; i++ {
			for j := 0; j < len(testContext.ringQ.Modulus)-nbRescals; j++ {
				require.Equalf(t, polWant.Coeffs[j][i], polTest.Coeffs[j][i], "coeff %v Qi%v = %s", i, j, coeffs[i].String())
			}
		}
	})
}

func testMarshalBinary(testContext *testParams, t *testing.T) {

	t.Run(testString("MarshalBinary/Ring/", testContext.ringQ), func(t *testing.T) {

		data, _ := testContext.ringQ.MarshalBinary()

		ringQTest := new(Ring)
		ringQTest.UnmarshalBinary(data)

		require.Equal(t, ringQTest.N, testContext.ringQ.N)
		require.Equal(t, ringQTest.Modulus, testContext.ringQ.Modulus)
	})

	t.Run(testString("MarshalBinary/Poly/", testContext.ringQ), func(t *testing.T) {

		p := testContext.uniformSamplerQ.ReadNew()
		pTest := testContext.ringQ.NewPoly()

		data, _ := p.MarshalBinary()

		_ = pTest.UnmarshalBinary(data)

		for i := range testContext.ringQ.Modulus {
			require.Equal(t, p.Coeffs[i][:testContext.ringQ.N], pTest.Coeffs[i][:testContext.ringQ.N])
		}
	})
}

func testUniformSampler(testContext *testParams, t *testing.T) {

	t.Run(testString("UniformSampler/Read/", testContext.ringQ), func(t *testing.T) {
		pol := testContext.ringQ.NewPoly()
		testContext.uniformSamplerQ.Read(pol)
		for i := uint64(0); i < testContext.ringQ.N; i++ {
			for j, qi := range testContext.ringQ.Modulus {
				require.False(t, pol.Coeffs[j][i] > qi)
			}
		}
	})

	t.Run(testString("UniformSampler/ReadNew/", testContext.ringQ), func(t *testing.T) {
		pol := testContext.uniformSamplerQ.ReadNew()
		for i := uint64(0); i < testContext.ringQ.N; i++ {
			for j, qi := range testContext.ringQ.Modulus {
				require.False(t, pol.Coeffs[j][i] > qi)
			}
		}
	})
}

func testGaussianSampler(testContext *testParams, t *testing.T) {

	t.Run(testString("GaussianSampler/", testContext.ringQ), func(t *testing.T) {
		gaussianSampler := NewGaussianSampler(testContext.prng, testContext.ringQ, DefaultSigma, DefaultBound)
		pol := gaussianSampler.ReadNew()

		for i := uint64(0); i < testContext.ringQ.N; i++ {
			for j, qi := range testContext.ringQ.Modulus {
				require.False(t, uint64(DefaultBound) < pol.Coeffs[j][i] && pol.Coeffs[j][i] < (qi-uint64(DefaultBound)))
			}
		}
	})
}

func testTernarySampler(testContext *testParams, t *testing.T) {

	for _, p := range []float64{.5, 1. / 3., 128. / 65536.} {
		t.Run(testString(fmt.Sprintf("TernarySampler/p=%1.2f/", p), testContext.ringQ), func(t *testing.T) {

			countOne := uint64(0)
			countZer := uint64(0)
			countMOne := uint64(0)

			pol := testContext.ringQ.NewPoly()
			prng, err := utils.NewPRNG()
			if err != nil {
				panic(err)
			}
			ternarySampler := NewTernarySampler(prng, testContext.ringQ, p, false)

			ternarySampler.Read(pol)

			for i := range pol.Coeffs[0] {
				if pol.Coeffs[0][i] == testContext.ringQ.Modulus[0]-1 {
					countMOne++
				}

				if pol.Coeffs[0][i] == 0 {
					countZer++
				}

				if pol.Coeffs[0][i] == 1 {
					countOne++
				}
			}

			N := float64(testContext.ringQ.N)
			pFail := math.Pow(2, -40)
			pNonZero := 1 - p
			POneMinOne := pNonZero / 2

			// Checks that the Hoeffding's inequality holds for non-zero elements count
			boundNonZero := (pNonZero - math.Sqrt(math.Log(pFail)/(-2*N))) * N
			require.GreaterOrEqual(t, float64(countOne+countMOne), boundNonZero)

			// Checks that the Hoeffding's inequality holds for one and minus one elements count
			boundOneMinOne := (POneMinOne - math.Sqrt(math.Log(pFail)/(-2*N))) * N
			require.GreaterOrEqual(t, float64(countOne), boundOneMinOne)
			require.GreaterOrEqual(t, float64(countMOne), boundOneMinOne)
		})
	}
}

func testModularReduction(testContext *testParams, t *testing.T) {

	t.Run(testString("ModularReduction/BRed/", testContext.ringQ), func(t *testing.T) {

		for j, q := range testContext.ringQ.Modulus {

			bigQ := NewUint(q)

			for i := 0; i < 65536; i++ {
				x := rand.Uint64() % q
				y := rand.Uint64() % q

				result := NewUint(x)
				result.Mul(result, NewUint(y))
				result.Mod(result, bigQ)

				require.Equalf(t, BRed(x, y, q, testContext.ringQ.BredParams[j]), result.Uint64(), "x = %v, y=%v", x, y)
			}
		}
	})

	t.Run(testString("ModularReduction/MRed/", testContext.ringQ), func(t *testing.T) {

		for j := range testContext.ringQ.Modulus {

			q := testContext.ringQ.Modulus[j]

			bigQ := NewUint(q)

			for i := 0; i < 65536; i++ {

				x := rand.Uint64() % q
				y := rand.Uint64() % q

				result := NewUint(x)
				result.Mul(result, NewUint(y))
				result.Mod(result, bigQ)

				require.Equalf(t, MRed(x, MForm(y, q, testContext.ringQ.BredParams[j]), q, testContext.ringQ.MredParams[j]), result.Uint64(), "x = %v, y=%v", x, y)
			}
		}
	})
}

func testGaloisShift(testContext *testParams, t *testing.T) {

	t.Run(testString("GaloisShift/", testContext.ringQ), func(t *testing.T) {

		pWant := testContext.uniformSamplerQ.ReadNew()
		pTest := pWant.CopyNew()

		testContext.ringQ.BitReverse(pTest, pTest)
		testContext.ringQ.InvNTT(pTest, pTest)
		testContext.ringQ.Rotate(pTest, 1, pTest)
		testContext.ringQ.NTT(pTest, pTest)
		testContext.ringQ.BitReverse(pTest, pTest)
		testContext.ringQ.Reduce(pTest, pTest)

		testContext.ringQ.Shift(pWant, 1, pWant)

		for i := range testContext.ringQ.Modulus {
			require.Equal(t, pTest.Coeffs[i][:testContext.ringQ.N], pWant.Coeffs[i][:testContext.ringQ.N])
		}
	})
}

func testMForm(testContext *testParams, t *testing.T) {

	t.Run(testString("MForm/", testContext.ringQ), func(t *testing.T) {

		polWant := testContext.uniformSamplerQ.ReadNew()
		polTest := testContext.ringQ.NewPoly()

		testContext.ringQ.MForm(polWant, polTest)
		testContext.ringQ.InvMForm(polTest, polTest)

		require.True(t, testContext.ringQ.Equal(polWant, polTest))
	})
}

func testMulScalarBigint(testContext *testParams, t *testing.T) {

	t.Run(testString("MulScalarBigint/", testContext.ringQ), func(t *testing.T) {

		polWant := testContext.uniformSamplerQ.ReadNew()
		polTest := polWant.CopyNew()

		rand1 := RandUniform(testContext.prng, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF)
		rand2 := RandUniform(testContext.prng, 0xFFFFFFFFFFFFFFFF, 0xFFFFFFFFFFFFFFFF)

		scalarBigint := NewUint(rand1)
		scalarBigint.Mul(scalarBigint, NewUint(rand2))

		testContext.ringQ.MulScalar(polWant, rand1, polWant)
		testContext.ringQ.MulScalar(polWant, rand2, polWant)
		testContext.ringQ.MulScalarBigint(polTest, scalarBigint, polTest)

		require.True(t, testContext.ringQ.Equal(polWant, polTest))
	})
}

func testMulPoly(testContext *testParams, t *testing.T) {

	p1 := testContext.uniformSamplerQ.ReadNew()
	p2 := testContext.uniformSamplerQ.ReadNew()
	p3Test := testContext.ringQ.NewPoly()
	p3Want := testContext.ringQ.NewPoly()

	testContext.ringQ.Reduce(p1, p1)
	testContext.ringQ.Reduce(p2, p2)

	testContext.ringQ.MulPolyNaive(p1, p2, p3Want)

	t.Run(testString("MulPoly/Barrett/", testContext.ringQ), func(t *testing.T) {

		testContext.ringQ.MulPoly(p1, p2, p3Test)

		require.Equal(t, p3Want.Coeffs[0][:testContext.ringQ.N], p3Test.Coeffs[0][:testContext.ringQ.N])
	})

	t.Run(testString("MulPoly/Montgomery/", testContext.ringQ), func(t *testing.T) {

		testContext.ringQ.MForm(p1, p1)
		testContext.ringQ.MForm(p2, p2)

		testContext.ringQ.MulPolyMontgomery(p1, p2, p3Test)

		testContext.ringQ.InvMForm(p3Test, p3Test)

		require.Equal(t, p3Want.Coeffs[0][:testContext.ringQ.N], p3Test.Coeffs[0][:testContext.ringQ.N])
	})
}

func testExtendBasis(testContext *testParams, t *testing.T) {

	t.Run(testString("ExtendBasis/", testContext.ringQ), func(t *testing.T) {

		basisextender := NewFastBasisExtender(testContext.ringQ, testContext.ringP)

		coeffs := make([]*big.Int, testContext.ringQ.N)
		for i := uint64(0); i < testContext.ringQ.N; i++ {
			coeffs[i] = RandInt(testContext.ringQ.ModulusBigint)
		}

		Pol := testContext.ringQ.NewPoly()
		PolTest := testContext.ringP.NewPoly()
		PolWant := testContext.ringP.NewPoly()

		testContext.ringQ.SetCoefficientsBigint(coeffs, Pol)
		testContext.ringP.SetCoefficientsBigint(coeffs, PolWant)

		basisextender.ModUpSplitQP(uint64(len(testContext.ringQ.Modulus)-1), Pol, PolTest)

		for i := range testContext.ringP.Modulus {
			require.Equal(t, PolTest.Coeffs[i][:testContext.ringQ.N], PolWant.Coeffs[i][:testContext.ringQ.N])
		}
	})
}

func testScaling(testContext *testParams, t *testing.T) {

	t.Run(testString("Scaling/Simple/", testContext.ringQ), func(t *testing.T) {

		rescaler := NewSimpleScaler(T, testContext.ringQ)

		coeffs := make([]*big.Int, testContext.ringQ.N)
		for i := uint64(0); i < testContext.ringQ.N; i++ {
			coeffs[i] = RandInt(testContext.ringQ.ModulusBigint)
		}

		coeffsWant := make([]*big.Int, testContext.ringQ.N)
		for i := range coeffs {
			coeffsWant[i] = new(big.Int).Set(coeffs[i])
			coeffsWant[i].Mul(coeffsWant[i], NewUint(T))
			DivRound(coeffsWant[i], testContext.ringQ.ModulusBigint, coeffsWant[i])
			coeffsWant[i].Mod(coeffsWant[i], NewUint(T))
		}

		PolTest := testContext.ringQ.NewPoly()

		testContext.ringQ.SetCoefficientsBigint(coeffs, PolTest)

		rescaler.DivByQOverTRounded(PolTest, PolTest)

		for i := uint64(0); i < testContext.ringQ.N; i++ {
			require.Equal(t, PolTest.Coeffs[0][i], coeffsWant[i].Uint64())
		}
	})

	t.Run(testString("Scaling/RNS", testContext.ringQ), func(t *testing.T) {

		scaler := NewRNSScaler(T, testContext.ringQ)

		coeffs := make([]*big.Int, testContext.ringQ.N)
		for i := uint64(0); i < testContext.ringQ.N; i++ {
			coeffs[i] = RandInt(testContext.ringQ.ModulusBigint)
		}

		coeffsWant := make([]*big.Int, testContext.ringQ.N)
		for i := range coeffs {
			coeffsWant[i] = new(big.Int).Set(coeffs[i])
			coeffsWant[i].Mul(coeffsWant[i], NewUint(T))
			DivRound(coeffsWant[i], testContext.ringQ.ModulusBigint, coeffsWant[i])
			coeffsWant[i].Mod(coeffsWant[i], NewUint(T))
		}

		polyQ := testContext.ringQ.NewPoly()
		polyT := NewPoly(testContext.ringQ.N, 1)
		testContext.ringQ.SetCoefficientsBigint(coeffs, polyQ)

		scaler.DivByQOverTRounded(polyQ, polyT)

		for i := uint64(0); i < testContext.ringQ.N; i++ {
			require.Equal(t, polyT.Coeffs[0][i], coeffsWant[i].Uint64())
		}
	})
}

func testMultByMonomial(testContext *testParams, t *testing.T) {

	t.Run(testString("MultByMonomial/", testContext.ringQ), func(t *testing.T) {

		p1 := testContext.uniformSamplerQ.ReadNew()

		p3Test := testContext.ringQ.NewPoly()
		p3Want := testContext.ringQ.NewPoly()

		testContext.ringQ.MultByMonomial(p1, 1, p3Test)
		testContext.ringQ.MultByMonomial(p3Test, 8, p3Test)

		testContext.ringQ.MultByMonomial(p1, 9, p3Want)

		require.Equal(t, p3Want.Coeffs[0][:testContext.ringQ.N], p3Test.Coeffs[0][:testContext.ringQ.N])
	})
}
