package ckks

import (
	"math"
)

// ChebyshevInterpolation is a struct storing the coefficients, degree and range of a Chebyshev interpolation polynomial.
type ChebyshevInterpolation struct {
	coeffs map[uint64]complex128
	degree uint64
	a      complex128
	b      complex128
}

// Approximate computes a Chebyshev approximation of the input function, for the range [-a, b] of degree degree.
// To be used in conjunction with the function EvaluateCheby.
func Approximate(function func(complex128) complex128, a, b complex128, degree int) (cheby *ChebyshevInterpolation) {

	cheby = new(ChebyshevInterpolation)
	cheby.coeffs = make(map[uint64]complex128)
	cheby.a = a
	cheby.b = b
	cheby.degree = uint64(degree)

	nodes := chebyshevNodes(degree+1, a, b)

	fi := make([]complex128, len(nodes))
	for i := range nodes {
		fi[i] = function(nodes[i])
	}

	coeffs := chebyCoeffs(nodes, fi, a, b)

	for i := range coeffs {
		cheby.coeffs[uint64(i)] = coeffs[i]
	}

	return
}

func chebyshevNodes(n int, a, b complex128) (u []complex128) {
	u = make([]complex128, n)
	var x, y complex128
	for k := 1; k < n+1; k++ {
		x = 0.5 * (a + b)
		y = 0.5 * (b - a)
		u[k-1] = x + y*complex(math.Cos((float64(k)-0.5)*(3.141592653589793/float64(n))), 0)
	}
	return
}

func evaluateChebyshevPolynomial(coeffs []complex128, x complex128, a, b complex128) (y complex128) {
	var u, Tprev, Tnext, T complex128
	u = (2*x - a - b) / (b - a)
	Tprev = 1
	T = u
	y = coeffs[0]
	for i := 1; i < len(coeffs); i++ {
		y = y + T*coeffs[i]
		Tnext = 2*u*T - Tprev
		Tprev = T
		T = Tnext
	}
	return
}

func chebyCoeffs(nodes, fi []complex128, a, b complex128) (coeffs []complex128) {

	var u, Tprev, T, Tnext complex128

	n := len(nodes)

	coeffs = make([]complex128, n)

	for i := 0; i < n; i++ {

		u = (2*nodes[i] - a - b) / (b - a)
		Tprev = 1
		T = u

		for j := 0; j < n; j++ {
			coeffs[j] += fi[i] * Tprev
			Tnext = 2*u*T - Tprev
			Tprev = T
			T = Tnext
		}
	}

	coeffs[0] /= complex(float64(n), 0)
	for i := 1; i < n; i++ {
		coeffs[i] *= (2.0 / complex(float64(n), 0))
	}

	return
}
