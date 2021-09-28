package ring

import (
	"math/big"
	"math/bits"
)

//============================
//=== Montgomery REDUCTION ===
//============================

// MForm returns a*2^64 mod q. It takes the input a in
// conventional form and returns r which is the
// the Montgomery form of a of a mod q with a radix of 2^64.
func MForm(a, q uint64, u []uint64) (r uint64) {
	mhi, _ := bits.Mul64(a, u[1])
	r = -(a*u[0] + mhi) * q
	if r >= q {
		r -= q
	}
	return
}

// MFormConstant is identical to MForm, except that it runs in constant time
// and returns a value in [0, 2q-1] (it omits the conditional reduction).
func MFormConstant(a, q uint64, u []uint64) (r uint64) {
	mhi, _ := bits.Mul64(a, u[1])
	r = -(a*u[0] + mhi) * q
	return
}

// InvMForm returns a*(1/2^64) mod q. It takes the input a in
// Montgomery form mod q with a radix of 2^64 and returns r which is the normal form of a mod q.
func InvMForm(a, q, qInv uint64) (r uint64) {
	r, _ = bits.Mul64(a*qInv, q)
	r = q - r
	if r >= q {
		r -= q
	}
	return
}

// InvMFormConstant is indentical to InvMForm, except that it runs in constant time
// and returns a value in [0, 2q-1].
func InvMFormConstant(a, q, qInv uint64) (r uint64) {
	r, _ = bits.Mul64(a*qInv, q)
	r = q - r
	return
}

// MRedParams computes the parameter qInv = (q^-1) mod 2^64,
// required for MRed.
func MRedParams(q uint64) (qInv uint64) {
	var x uint64
	qInv = 1
	x = q
	for i := 0; i < 63; i++ {
		qInv *= x
		qInv &= 0xFFFFFFFFFFFFFFFF
		x *= x
		x &= 0xFFFFFFFFFFFFFFFF
	}
	return
}

// MRed computes x * y * (1/2^64) mod q. Requires that at least one of the inputs is in
// Montgomery form. If only one of the inputs is in Montgomery form (ex : a pre-computed constant),
// the result will be in normal form. If both inputs are in Montgomery form, then the result
// will be in Montgomery form.
func MRed(x, y, q, qInv uint64) (r uint64) {
	ahi, alo := bits.Mul64(x, y)
	R := alo * qInv
	H, _ := bits.Mul64(R, q)
	r = ahi - H + q
	if r >= q {
		r -= q
	}
	return
}

// MRedConstant is identical to MRed except it runs in
// constant time and returns a value in [0, 2q-1].
func MRedConstant(x, y, q, qInv uint64) (r uint64) {
	ahi, alo := bits.Mul64(x, y)
	R := alo * qInv
	H, _ := bits.Mul64(R, q)
	r = ahi - H + q
	return
}

//==========================
//=== BARRETT REDUCTION  ===
//==========================

// BRedParams computes the parameters required for the BRed with
// a radix of 2^128.
func BRedParams(q uint64) (params []uint64) {
	bigR := new(big.Int).Lsh(NewUint(1), 128)
	bigR.Quo(bigR, NewUint(q))

	// 2^radix // q
	mhi := new(big.Int).Rsh(bigR, 64).Uint64()
	mlo := bigR.Uint64()

	return []uint64{mhi, mlo}
}

// BRedAdd reduces a 64 bit integer by q.
// Assumes that x <= 64bits. Useful when several additions
// are performed before a modular reduction, as it is faster than
// applying a conditional reduction after each addition.
func BRedAdd(x, q uint64, u []uint64) (r uint64) {
	s0, _ := bits.Mul64(x, u[0])
	r = x - s0*q
	if r >= q {
		r -= q
	}
	return
}

// BRedAddConstant is indentical to BReAdd, except it runs
// in constant time and returns a value in [0, 2q-1].
func BRedAddConstant(x, q uint64, u []uint64) uint64 {
	s0, _ := bits.Mul64(x, u[0])
	return x - s0*q
}

// BRed compute x*y mod q for arbitrary x,y uint64. To be used
// when both x,y can not be pre-computed. However applying a Montgomery
// transform on either a or b might be faster depending on the computation
// to do, especially if either x or y needs to be multiplied with several other
// values.
func BRed(x, y, q uint64, u []uint64) (r uint64) {

	var lhi, mhi, mlo, s0, s1, carry uint64

	ahi, alo := bits.Mul64(x, y)

	// (alo*ulo)>>64

	lhi, _ = bits.Mul64(alo, u[1])

	// ((ahi*ulo + alo*uhi) + (alo*ulo))>>64

	mhi, mlo = bits.Mul64(alo, u[0])

	s0, carry = bits.Add64(mlo, lhi, 0)

	s1 = mhi + carry

	mhi, mlo = bits.Mul64(ahi, u[1])

	_, carry = bits.Add64(mlo, s0, 0)

	lhi = mhi + carry

	// (ahi*uhi) + (((ahi*ulo + alo*uhi) + (alo*ulo))>>64)

	s0 = ahi*u[0] + s1 + lhi

	r = alo - s0*q

	if r >= q {
		r -= q
	}

	return
}

// BRedConstant is indentical to BRed, except it runs
// in constant time and returns a value in [0, 2q-1].
func BRedConstant(x, y, q uint64, u []uint64) (r uint64) {

	var lhi, mhi, mlo, s0, s1, carry uint64

	ahi, alo := bits.Mul64(x, y)

	// alo*ulo

	lhi, _ = bits.Mul64(alo, u[1])

	// ahi*ulo + alo*uhi

	mhi, mlo = bits.Mul64(alo, u[0])

	s0, carry = bits.Add64(mlo, lhi, 0)

	s1 = mhi + carry

	mhi, mlo = bits.Mul64(ahi, u[1])

	_, carry = bits.Add64(mlo, s0, 0)

	lhi = mhi + carry

	// ahi*uhi

	s0 = ahi*u[0] + s1 + lhi

	r = alo - s0*q

	return
}

//===============================
//==== CONDITIONAL REDUCTION ====
//===============================

// CRed reduce returns a mod q, where
// a is required to be in the range [0, 2q-1].
func CRed(a, q uint64) uint64 {
	if a >= q {
		return a - q
	}
	return a
}
