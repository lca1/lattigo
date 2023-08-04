package circuits

import (
	"fmt"
	"math/big"
	"math/bits"

	"github.com/tuneinsight/lattigo/v4/ckks"
	"github.com/tuneinsight/lattigo/v4/rlwe"
	"github.com/tuneinsight/lattigo/v4/utils"
	"github.com/tuneinsight/lattigo/v4/utils/bignum"
)

type CKKSPolyEvaluator struct {
	PolynomialEvaluator
	Parameters ckks.Parameters
}

// NewCKKSPowerBasis is a wrapper of NewPolynomialBasis.
// This function creates a new powerBasis from the input ciphertext.
// The input ciphertext is treated as the base monomial X used to
// generate the other powers X^{n}.
func NewCKKSPowerBasis(ct *rlwe.Ciphertext, basis bignum.Basis) PowerBasis {
	return NewPowerBasis(ct, basis)
}

// NewCKKSPolynomial is a wrapper of NewPolynomial.
// This function creates a new polynomial from the input coefficients.
// This polynomial can be evaluated on a ciphertext.
func NewCKKSPolynomial(poly bignum.Polynomial) Polynomial {
	return NewPolynomial(poly)
}

// NewCKKSPolynomialVector is a wrapper of NewPolynomialVector.
// This function creates a new PolynomialVector from the input polynomials and the desired function mapping.
// This polynomial vector can be evaluated on a ciphertext.
func NewCKKSPolynomialVector(polys []Polynomial, mapping map[int][]int) (PolynomialVector, error) {
	return NewPolynomialVector(polys, mapping)
}

func NewCKKSPolynomialEvaluator(params ckks.Parameters, eval EvaluatorForPolyEval) *CKKSPolyEvaluator {
	e := new(CKKSPolyEvaluator)
	e.PolynomialEvaluator = PolynomialEvaluator{eval, eval.GetEvaluatorBuffer()}
	e.Parameters = params
	return e
}

// Polynomial evaluates a polynomial in standard basis on the input Ciphertext in ceil(log2(deg+1)) levels.
// Returns an error if the input ciphertext does not have enough level to carry out the full polynomial evaluation.
// Returns an error if something is wrong with the scale.
// If the polynomial is given in Chebyshev basis, then a change of basis ct' = (2/(b-a)) * (ct + (-a-b)/(b-a))
// is necessary before the polynomial evaluation to ensure correctness.
// input must be either *rlwe.Ciphertext or *PolynomialBasis.
// pol: a *bignum.Polynomial, *Polynomial or *PolynomialVector
// targetScale: the desired output scale. This value shouldn't differ too much from the original ciphertext scale. It can
// for example be used to correct small deviations in the ciphertext scale and reset it to the default scale.
func (eval CKKSPolyEvaluator) Polynomial(input interface{}, p interface{}, targetScale rlwe.Scale) (opOut *rlwe.Ciphertext, err error) {

	var polyVec PolynomialVector
	switch p := p.(type) {
	case bignum.Polynomial:
		polyVec = PolynomialVector{Value: []Polynomial{{Polynomial: p, MaxDeg: p.Degree(), Lead: true, Lazy: false}}}
	case Polynomial:
		polyVec = PolynomialVector{Value: []Polynomial{p}}
	case PolynomialVector:
		polyVec = p
	default:
		return nil, fmt.Errorf("cannot Polynomial: invalid polynomial type: %T", p)
	}

	var powerbasis PowerBasis
	switch input := input.(type) {
	case *rlwe.Ciphertext:
		powerbasis = NewPowerBasis(input, polyVec.Value[0].Basis)
	case PowerBasis:
		if input.Value[1] == nil {
			return nil, fmt.Errorf("cannot evaluatePolyVector: given PowerBasis.Value[1] is empty")
		}
		powerbasis = input
	default:
		return nil, fmt.Errorf("cannot evaluatePolyVector: invalid input, must be either *rlwe.Ciphertext or *PowerBasis")
	}

	levelsConsummedPerRescaling := eval.Parameters.LevelsConsummedPerRescaling()

	if err := checkEnoughLevels(powerbasis.Value[1].Level(), levelsConsummedPerRescaling*polyVec.Value[0].Depth()); err != nil {
		return nil, err
	}

	logDegree := bits.Len64(uint64(polyVec.Value[0].Degree()))
	logSplit := bignum.OptimalSplit(logDegree)

	var odd, even = false, false
	for _, p := range polyVec.Value {
		odd, even = odd || p.IsOdd, even || p.IsEven
	}

	// Computes all the powers of two with relinearization
	// This will recursively compute and store all powers of two up to 2^logDegree
	if err = powerbasis.GenPower(1<<(logDegree-1), false, eval); err != nil {
		return nil, err
	}

	// Computes the intermediate powers, starting from the largest, without relinearization if possible
	for i := (1 << logSplit) - 1; i > 2; i-- {
		if !(even || odd) || (i&1 == 0 && even) || (i&1 == 1 && odd) {
			if err = powerbasis.GenPower(i, polyVec.Value[0].Lazy, eval); err != nil {
				return nil, err
			}
		}
	}

	params := *eval.GetRLWEParameters()

	PS := polyVec.GetPatersonStockmeyerPolynomial(params, powerbasis.Value[1].Level(), powerbasis.Value[1].Scale, targetScale, &ckksDummyEvaluator{params, levelsConsummedPerRescaling})

	if opOut, err = eval.EvaluatePatersonStockmeyerPolynomialVector(eval, PS, powerbasis); err != nil {
		return nil, err
	}

	return opOut, err
}

type ckksDummyEvaluator struct {
	params                      rlwe.Parameters
	levelsConsummedPerRescaling int
}

func (d ckksDummyEvaluator) PolynomialDepth(degree int) int {
	return d.levelsConsummedPerRescaling * (bits.Len64(uint64(degree)) - 1)
}

// Rescale rescales the target DummyOperand n times and returns it.
func (d ckksDummyEvaluator) Rescale(op0 *DummyOperand) {
	for i := 0; i < d.levelsConsummedPerRescaling; i++ {
		op0.Scale = op0.Scale.Div(rlwe.NewScale(d.params.Q()[op0.Level]))
		op0.Level--
	}
}

// Mul multiplies two DummyOperand, stores the result the taret DummyOperand and returns the result.
func (d ckksDummyEvaluator) MulNew(op0, op1 *DummyOperand) (opOut *DummyOperand) {
	opOut = new(DummyOperand)
	opOut.Level = utils.Min(op0.Level, op1.Level)
	opOut.Scale = op0.Scale.Mul(op1.Scale)
	return
}

func (d ckksDummyEvaluator) UpdateLevelAndScaleBabyStep(lead bool, tLevelOld int, tScaleOld rlwe.Scale) (tLevelNew int, tScaleNew rlwe.Scale) {

	tLevelNew = tLevelOld
	tScaleNew = tScaleOld

	if lead {
		for i := 0; i < d.levelsConsummedPerRescaling; i++ {
			tScaleNew = tScaleNew.Mul(rlwe.NewScale(d.params.Q()[tLevelNew-i]))
		}
	}

	return
}

func (d ckksDummyEvaluator) UpdateLevelAndScaleGiantStep(lead bool, tLevelOld int, tScaleOld, xPowScale rlwe.Scale) (tLevelNew int, tScaleNew rlwe.Scale) {

	Q := d.params.Q()

	var qi *big.Int
	if lead {
		qi = bignum.NewInt(Q[tLevelOld])
		for i := 1; i < d.levelsConsummedPerRescaling; i++ {
			qi.Mul(qi, bignum.NewInt(Q[tLevelOld-i]))
		}
	} else {
		qi = bignum.NewInt(Q[tLevelOld+d.levelsConsummedPerRescaling])
		for i := 1; i < d.levelsConsummedPerRescaling; i++ {
			qi.Mul(qi, bignum.NewInt(Q[tLevelOld+d.levelsConsummedPerRescaling-i]))
		}
	}

	tLevelNew = tLevelOld + d.levelsConsummedPerRescaling
	tScaleNew = tScaleOld.Mul(rlwe.NewScale(qi))
	tScaleNew = tScaleNew.Div(xPowScale)

	return
}

func (d ckksDummyEvaluator) GetPolynmialDepth(degree int) int {
	return d.levelsConsummedPerRescaling * (bits.Len64(uint64(degree)) - 1)
}

func (eval CKKSPolyEvaluator) EvaluatePolynomialVectorFromPowerBasis(targetLevel int, pol PolynomialVector, pb PowerBasis, targetScale rlwe.Scale) (res *rlwe.Ciphertext, err error) {

	// Map[int] of the powers [X^{0}, X^{1}, X^{2}, ...]
	X := pb.Value

	// Retrieve the number of slots
	logSlots := X[1].LogDimensions
	slots := 1 << logSlots.Cols

	params := eval.Parameters
	slotsIndex := pol.SlotsIndex
	even := pol.IsEven()
	odd := pol.IsOdd()

	// Retrieve the degree of the highest degree non-zero coefficient
	// TODO: optimize for nil/zero coefficients
	minimumDegreeNonZeroCoefficient := len(pol.Value[0].Coeffs) - 1
	if even && !odd {
		minimumDegreeNonZeroCoefficient--
	}

	// Gets the maximum degree of the ciphertexts among the power basis
	// TODO: optimize for nil/zero coefficients, odd/even polynomial
	maximumCiphertextDegree := 0
	for i := pol.Value[0].Degree(); i > 0; i-- {
		if x, ok := X[i]; ok {
			maximumCiphertextDegree = utils.Max(maximumCiphertextDegree, x.Degree())
		}
	}

	// If an index slot is given (either multiply polynomials or masking)
	if slotsIndex != nil {

		var toEncode bool

		// Allocates temporary buffer for coefficients encoding
		values := make([]*bignum.Complex, slots)

		// If the degree of the poly is zero
		if minimumDegreeNonZeroCoefficient == 0 {

			// Allocates the output ciphertext
			res = ckks.NewCiphertext(params, 1, targetLevel)
			res.Scale = targetScale
			res.LogDimensions = logSlots

			// Looks for non-zero coefficients among the degree 0 coefficients of the polynomials
			if even {
				for i, p := range pol.Value {
					if !isZero(p.Coeffs[0]) {
						toEncode = true
						for _, j := range slotsIndex[i] {
							values[j] = p.Coeffs[0]
						}
					}
				}
			}

			// If a non-zero coefficient was found, encode the values, adds on the ciphertext, and returns
			if toEncode {
				pt := &rlwe.Plaintext{}
				pt.Value = res.Value[0]
				pt.MetaData = res.MetaData
				if err = eval.Encode(values, pt); err != nil {
					return nil, err
				}
			}

			return
		}

		// Allocates the output ciphertext
		res = ckks.NewCiphertext(params, maximumCiphertextDegree, targetLevel)
		res.Scale = targetScale
		res.LogDimensions = logSlots

		// Looks for a non-zero coefficient among the degree zero coefficient of the polynomials
		if even {
			for i, p := range pol.Value {
				if !isZero(p.Coeffs[0]) {
					toEncode = true
					for _, j := range slotsIndex[i] {
						values[j] = p.Coeffs[0]
					}
				}
			}
		}

		// If a non-zero degre coefficient was found, encode and adds the values on the output
		// ciphertext
		if toEncode {
			if err = eval.Add(res, values, res); err != nil {
				return
			}
			toEncode = false
		}

		// Loops starting from the highest degree coefficient
		for key := pol.Value[0].Degree(); key > 0; key-- {

			var reset bool

			if !(even || odd) || (key&1 == 0 && even) || (key&1 == 1 && odd) {

				// Loops over the polynomials
				for i, p := range pol.Value {

					// Looks for a non-zero coefficient
					if !isZero(p.Coeffs[key]) {
						toEncode = true

						// Resets the temporary array to zero
						// is needed if a zero coefficient
						// is at the place of a previous non-zero
						// coefficient
						if !reset {
							for j := range values {
								if values[j] != nil {
									values[j][0].SetFloat64(0)
									values[j][1].SetFloat64(0)
								}
							}
							reset = true
						}

						// Copies the coefficient on the temporary array
						// according to the slot map index
						for _, j := range slotsIndex[i] {
							values[j] = p.Coeffs[key]
						}
					}
				}
			}

			// If a non-zero degre coefficient was found, encode and adds the values on the output
			// ciphertext
			if toEncode {
				if err = eval.MulThenAdd(X[key], values, res); err != nil {
					return
				}
				toEncode = false
			}
		}

	} else {

		var c *bignum.Complex
		if even && !isZero(pol.Value[0].Coeffs[0]) {
			c = pol.Value[0].Coeffs[0]
		}

		if minimumDegreeNonZeroCoefficient == 0 {

			res = ckks.NewCiphertext(params, 1, targetLevel)
			res.Scale = targetScale
			res.LogDimensions = logSlots

			if !isZero(c) {
				if err = eval.Add(res, c, res); err != nil {
					return
				}
			}

			return
		}

		res = ckks.NewCiphertext(params, maximumCiphertextDegree, targetLevel)
		res.Scale = targetScale
		res.LogDimensions = logSlots

		if c != nil {
			if err = eval.Add(res, c, res); err != nil {
				return
			}
		}

		for key := pol.Value[0].Degree(); key > 0; key-- {
			if c = pol.Value[0].Coeffs[key]; key != 0 && !isZero(c) && (!(even || odd) || (key&1 == 0 && even) || (key&1 == 1 && odd)) {
				if err = eval.MulThenAdd(X[key], c, res); err != nil {
					return
				}
			}
		}
	}

	return
}

func isZero(c *bignum.Complex) bool {
	zero := new(big.Float)
	return c == nil || (c[0].Cmp(zero) == 0 && c[1].Cmp(zero) == 0)
}

// checkEnoughLevels checks that enough levels are available to evaluate the bignum.
// Also checks if c is a Gaussian integer or not. If not, then one more level is needed
// to evaluate the bignum.
func checkEnoughLevels(levels, depth int) (err error) {

	if levels < depth {
		return fmt.Errorf("%d levels < %d log(d) -> cannot evaluate", levels, depth)
	}

	return nil
}