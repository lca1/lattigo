package main

import (
	"fmt"
	"math"
	"math/cmplx"
	"time"

	"github.com/tuneinsight/lattigo/v4/core/rlwe"
	"github.com/tuneinsight/lattigo/v4/he"
	"github.com/tuneinsight/lattigo/v4/he/hefloat"
	"github.com/tuneinsight/lattigo/v4/utils/bignum"
)

func example() {

	var start time.Time
	var err error

	// Schemes parameters are created from scratch
	params, err := hefloat.NewParametersFromLiteral(
		hefloat.ParametersLiteral{
			LogN:            14,
			LogQ:            []int{55, 40, 40, 40, 40, 40, 40, 40},
			LogP:            []int{45, 45},
			LogDefaultScale: 40,
		})
	if err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Println("=========================================")
	fmt.Println("         INSTANTIATING SCHEME            ")
	fmt.Println("=========================================")
	fmt.Println()

	start = time.Now()

	kgen := rlwe.NewKeyGenerator(params)

	sk := kgen.GenSecretKeyNew()

	encryptor := rlwe.NewEncryptor(params, sk)
	decryptor := rlwe.NewDecryptor(params, sk)
	encoder := hefloat.NewEncoder(params)
	evk := rlwe.NewMemEvaluationKeySet(kgen.GenRelinearizationKeyNew(sk))
	evaluator := hefloat.NewEvaluator(params, evk)

	fmt.Printf("Done in %s \n", time.Since(start))

	logSlots := params.LogMaxSlots()
	slots := 1 << logSlots

	fmt.Println()
	fmt.Printf("Scheme parameters: logN = %d, logSlots = %d, logQP = %f, levels = %d, scale= %f, noise = %T %v \n", params.LogN(), logSlots, params.LogQP(), params.MaxLevel()+1, params.DefaultScale().Float64(), params.Xe(), params.Xe())

	fmt.Println()
	fmt.Println("=========================================")
	fmt.Println("           PLAINTEXT CREATION            ")
	fmt.Println("=========================================")
	fmt.Println()

	start = time.Now()

	r := float64(16)

	pi := 3.141592653589793

	values := make([]complex128, slots)
	for i := range values {
		values[i] = complex(2*pi, 0)
	}

	plaintext := hefloat.NewPlaintext(params, params.MaxLevel())
	plaintext.Scale = plaintext.Scale.Div(rlwe.NewScale(r))
	if err := encoder.Encode(values, plaintext); err != nil {
		panic(err)
	}

	fmt.Printf("Done in %s \n", time.Since(start))

	fmt.Println()
	fmt.Println("=========================================")
	fmt.Println("              ENCRYPTION                 ")
	fmt.Println("=========================================")
	fmt.Println()

	start = time.Now()

	ciphertext, err := encryptor.EncryptNew(plaintext)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Done in %s \n", time.Since(start))

	printDebug(params, ciphertext, values, decryptor, encoder)

	fmt.Println()
	fmt.Println("===============================================")
	fmt.Printf("        EVALUATION OF i*x on %d values\n", slots)
	fmt.Println("===============================================")
	fmt.Println()

	start = time.Now()

	if err := evaluator.Mul(ciphertext, 1i, ciphertext); err != nil {
		panic(err)
	}

	fmt.Printf("Done in %s \n", time.Since(start))

	for i := range values {
		values[i] *= 1i
	}

	printDebug(params, ciphertext, values, decryptor, encoder)

	fmt.Println()
	fmt.Println("===============================================")
	fmt.Printf("       EVALUATION of x/r on %d values\n", slots)
	fmt.Println("===============================================")
	fmt.Println()

	start = time.Now()

	ciphertext.Scale = ciphertext.Scale.Mul(rlwe.NewScale(r))

	fmt.Printf("Done in %s \n", time.Since(start))

	for i := range values {
		values[i] /= complex(r, 0)
	}

	printDebug(params, ciphertext, values, decryptor, encoder)

	fmt.Println()
	fmt.Println("===============================================")
	fmt.Printf("       EVALUATION of e^x on %d values\n", slots)
	fmt.Println("===============================================")
	fmt.Println()

	start = time.Now()

	coeffs := []complex128{
		1.0,
		1.0,
		1.0 / 2,
		1.0 / 6,
		1.0 / 24,
		1.0 / 120,
		1.0 / 720,
		1.0 / 5040,
	}

	// We create a new polynomial, with the standard basis [1, x, x^2, ...], with no interval.
	poly := bignum.NewPolynomial(bignum.Monomial, coeffs, nil)

	polyEval := hefloat.NewPolynomialEvaluator(params, evaluator)

	if ciphertext, err = polyEval.Evaluate(ciphertext, poly, ciphertext.Scale); err != nil {
		panic(err)
	}

	fmt.Printf("Done in %s \n", time.Since(start))

	for i := range values {
		values[i] = cmplx.Exp(values[i])
	}

	printDebug(params, ciphertext, values, decryptor, encoder)

	fmt.Println()
	fmt.Println("===============================================")
	fmt.Printf("       EVALUATION of x^r on %d values\n", slots)
	fmt.Println("===============================================")
	fmt.Println()

	start = time.Now()

	monomialBasis := he.NewPowerBasis(ciphertext, bignum.Monomial)
	if err = monomialBasis.GenPower(int(r), false, evaluator); err != nil {
		panic(err)
	}
	ciphertext = monomialBasis.Value[int(r)]

	fmt.Printf("Done in %s \n", time.Since(start))

	for i := range values {
		values[i] = cmplx.Pow(values[i], complex(r, 0))
	}

	printDebug(params, ciphertext, values, decryptor, encoder)

	fmt.Println()
	fmt.Println("=========================================")
	fmt.Println("         DECRYPTION & DECODING           ")
	fmt.Println("=========================================")
	fmt.Println()

	start = time.Now()

	fmt.Printf("Done in %s \n", time.Since(start))

	printDebug(params, ciphertext, values, decryptor, encoder)

}

func printDebug(params hefloat.Parameters, ciphertext *rlwe.Ciphertext, valuesWant []complex128, decryptor *rlwe.Decryptor, encoder *hefloat.Encoder) (valuesTest []complex128) {

	valuesTest = make([]complex128, ciphertext.Slots())

	if err := encoder.Decode(decryptor.DecryptNew(ciphertext), valuesTest); err != nil {
		panic(err)
	}

	fmt.Println()
	fmt.Printf("Level: %d (logQ = %d)\n", ciphertext.Level(), params.LogQLvl(ciphertext.Level()))
	fmt.Printf("Scale: 2^%f\n", math.Log2(ciphertext.Scale.Float64()))
	fmt.Printf("ValuesTest: %6.10f %6.10f %6.10f %6.10f...\n", valuesTest[0], valuesTest[1], valuesTest[2], valuesTest[3])
	fmt.Printf("ValuesWant: %6.10f %6.10f %6.10f %6.10f...\n", valuesWant[0], valuesWant[1], valuesWant[2], valuesWant[3])
	fmt.Println()

	precStats := hefloat.GetPrecisionStats(params, encoder, nil, valuesWant, valuesTest, 0, false)

	fmt.Println(precStats.String())

	return
}

func main() {
	example()
}