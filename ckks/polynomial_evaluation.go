package ckks

import (
	"math"
	"math/bits"
)

// EvaluatePolyFast evaluates the polynomial a + bx + cx^2... on the input Ciphertext.
// It is faster than EvaluatePolyEco, but consumes ceil(log2(deg)) + 1 levels.
func (eval *evaluator) EvaluatePolyFast(ct0 *Ciphertext, coeffs interface{}, evakey *EvaluationKey) (ctOut *Ciphertext) {

	degree, coeffsMap := convertCoeffs(coeffs)

	C := make(map[uint64]*Ciphertext)

	C[1] = ct0.CopyNew().Ciphertext()

	M := uint64(bits.Len64(degree - 1))
	L := uint64(M >> 1)

	for i := uint64(2); i <= (1 << L); i++ {
		computePowerBasis(i, C, eval, evakey)
	}

	for i := L + 1; i < M; i++ {
		computePowerBasis(1<<i, C, eval, evakey)
	}

	return recurse(degree, L, M, coeffsMap, C, eval, evakey)
}

// EvaluatePolyEco evaluates the polynomial a + bx + cx^2... on the input Ciphertext.
// It is slower than EvaluatePolyFast, but it consumes one less level (ceil(log2(deg))).
func (eval *evaluator) EvaluatePolyEco(ct0 *Ciphertext, coeffs interface{}, evakey *EvaluationKey) (ctOut *Ciphertext) {

	degree, coeffsMap := convertCoeffs(coeffs)

	C := make(map[uint64]*Ciphertext)

	C[1] = ct0.CopyNew().Ciphertext()

	M := uint64(bits.Len64(degree - 1))
	L := uint64(1)

	for i := uint64(2); i <= (1 << L); i++ {
		computePowerBasis(i, C, eval, evakey)
	}

	for i := L + 1; i < M; i++ {
		computePowerBasis(1<<i, C, eval, evakey)
	}

	return recurse(degree, L, M, coeffsMap, C, eval, evakey)
}

func convertCoeffs(coeffs interface{}) (degree uint64, coeffsMap map[uint64]complex128) {

	coeffsMap = make(map[uint64]complex128)

	switch coeffs.(type) {
	case []complex128:
		for i := range coeffs.([]complex128) {
			coeffsMap[uint64(i)] = coeffs.([]complex128)[i]
		}
	case []float64:
		for i := range coeffs.([]float64) {
			coeffsMap[uint64(i)] = complex(coeffs.([]float64)[i], 0)
		}
	default:
		panic("cannot convertCoeffs: coeffs type must be complex128 or float64")
	}

	return uint64(len(coeffsMap)) - 1, coeffsMap
}

func computePowerBasis(n uint64, C map[uint64]*Ciphertext, evaluator *evaluator, evakey *EvaluationKey) {

	if C[n] == nil {

		// Computes the index required to compute the asked ring evaluation
		a := uint64(math.Ceil(float64(n) / 2))
		b := n >> 1

		// Recurses on the given indexes
		computePowerBasis(a, C, evaluator, evakey)
		computePowerBasis(b, C, evaluator, evakey)

		// Computes C[n] = C[a]*C[b]
		C[n] = evaluator.MulRelinNew(C[a], C[b], evakey)

		evaluator.Rescale(C[n], evaluator.ckksContext.scale, C[n])
	}
}

func splitCoeffs(coeffs map[uint64]complex128, degree, maxDegree uint64) (coeffsq, coeffsr map[uint64]complex128) {

	// Splits a polynomial p such that p = q*C^degree + r.

	coeffsr = make(map[uint64]complex128)
	coeffsq = make(map[uint64]complex128)

	for i := uint64(0); i < degree; i++ {
		coeffsr[i] = coeffs[i]
	}

	coeffsq[0] = coeffs[degree]

	for i := uint64(degree + 1); i < maxDegree+1; i++ {
		coeffsq[i-degree] = coeffs[i]
	}

	return coeffsq, coeffsr
}

func recurse(maxDegree, L, M uint64, coeffs map[uint64]complex128, C map[uint64]*Ciphertext, evaluator *evaluator, evakey *EvaluationKey) (res *Ciphertext) {

	if maxDegree <= (1 << L) {
		return evaluatePolyFromPowerBasis(coeffs, C, evaluator, evakey)
	}

	for 1<<(M-1) > maxDegree {
		M--
	}

	coeffsq, coeffsr := splitCoeffs(coeffs, 1<<(M-1), maxDegree)

	res = recurse(maxDegree-(1<<(M-1)), L, M-1, coeffsq, C, evaluator, evakey)

	var tmp *Ciphertext
	tmp = recurse((1<<(M-1))-1, L, M-1, coeffsr, C, evaluator, evakey)

	evaluator.MulRelin(res, C[1<<(M-1)], evakey, res)

	evaluator.Add(res, tmp, res)

	evaluator.Rescale(res, evaluator.ckksContext.scale, res)

	return res

}

func evaluatePolyFromPowerBasis(coeffs map[uint64]complex128, C map[uint64]*Ciphertext, evaluator *evaluator, evakey *EvaluationKey) (res *Ciphertext) {

	res = NewCiphertext(evaluator.params, 1, C[1].Level(), C[1].Scale())

	if math.Abs(real(coeffs[0])) > 1e-15 || math.Abs(imag(coeffs[0])) > 1e-15 {
		evaluator.AddConst(res, coeffs[0], res)
	}

	for key := range coeffs {
		if key != 0 && (math.Abs(real(coeffs[key])) > 1e-15 || math.Abs(imag(coeffs[key])) > 1e-15) {
			evaluator.MultByConstAndAdd(C[key], coeffs[key], res)
		}
	}

	evaluator.Rescale(res, evaluator.ckksContext.scale, res)

	return
}
