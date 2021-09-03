package rlwe

import (
	"github.com/ldsec/lattigo/v2/ring"
)

// PolyQP represents a polynomial in the ring of polynomial modulo Q*P.
// This type is simply the union type between two ring.Poly, each one
// containing the modulus Q and P coefficients of that polynomial.
// The modulus Q represent the ciphertext modulus and the modulus P
// the special primes for the RNS decomposition during homomorphic
// operations involving keys.
type PolyQP struct {
	Q, P *ring.Poly
}

// Equals returns true if the receiver PolyQP is equal to the provided other PolyQP.
// This method checks for equality of its two sub-polynomials.
func (p *PolyQP) Equals(other PolyQP) bool {
	return p == &other || (p.P.Equals(other.P) && p.Q.Equals(other.Q))
}

// CopyValues copies the coefficients of p1 on the target polynomial.
// This method simply calls the CopyValues method for each of its sub-polynomials.
func (p *PolyQP) CopyValues(other PolyQP) {
	p.Q.CopyValues(other.Q)
	p.P.CopyValues(other.P)
}

// CopyNew creates an exact copy of the target polynomial.
func (p *PolyQP) CopyNew() PolyQP {
	if p == nil {
		return PolyQP{}
	}
	return PolyQP{Q: p.Q.CopyNew(), P: p.P.CopyNew()}
}

// RingQP is a structure that implements the operation in the ring R_QP.
// This type is simply a union type between the two Ring types representing
// R_Q and R_P.
type RingQP struct {
	RingQ, RingP *ring.Ring
}

// NewPoly creates a new polynomial with all coefficients set to 0.
func (r *RingQP) NewPoly() PolyQP {
	return PolyQP{r.RingQ.NewPoly(), r.RingP.NewPoly()}
}

// NewPolyLvl creates a new polynomial with all coefficients set to 0.
func (r *RingQP) NewPolyLvl(levelQ, levelP int) PolyQP {
	return PolyQP{r.RingQ.NewPolyLvl(levelQ), r.RingP.NewPolyLvl(levelP)}
}

// AddLvl adds p1 to p2 coefficient-wise and writes the result on p3.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
func (r *RingQP) AddLvl(levelQ, levelP int, p1, p2, pOut PolyQP) {
	r.RingQ.AddLvl(levelQ, p1.Q, p2.Q, pOut.Q)
	r.RingP.AddLvl(levelP, p1.P, p2.P, pOut.P)
}

// SubLvl subtracts p2 to p1 coefficient-wise and writes the result on p3.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
func (r *RingQP) SubLvl(levelQ, levelP int, p1, p2, pOut PolyQP) {
	r.RingQ.SubLvl(levelQ, p1.Q, p2.Q, pOut.Q)
	r.RingP.SubLvl(levelP, p1.P, p2.P, pOut.P)
}

// NTTLvl computes the NTT of p1 and returns the result on p2.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
func (r *RingQP) NTTLvl(levelQ, levelP int, p, pOut PolyQP) {
	r.RingQ.NTTLvl(levelQ, p.Q, pOut.Q)
	r.RingP.NTTLvl(levelP, p.P, pOut.P)
}

// InvNTTLvl computes the inverse-NTT of p1 and returns the result on p2.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
func (r *RingQP) InvNTTLvl(levelQ, levelP int, p, pOut PolyQP) {
	r.RingQ.InvNTTLvl(levelQ, p.Q, pOut.Q)
	r.RingP.InvNTTLvl(levelP, p.P, pOut.P)
}

// NTTLazyLvl computes the NTT of p1 and returns the result on p2.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
// Output values are in the range [0, 2q-1].
func (r *RingQP) NTTLazyLvl(levelQ, levelP int, p, pOut PolyQP) {
	r.RingQ.NTTLazyLvl(levelQ, p.Q, pOut.Q)
	r.RingP.NTTLazyLvl(levelP, p.P, pOut.P)
}

// MFormLvl switches p1 to the Montgomery domain and writes the result on p2.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
func (r *RingQP) MFormLvl(levelQ, levelP int, p, pOut PolyQP) {
	r.RingQ.MFormLvl(levelQ, p.Q, pOut.Q)
	r.RingP.MFormLvl(levelP, p.P, pOut.P)
}

// InvMFormLvl switches back p1 from the Montgomery domain to the conventional domain and writes the result on p2.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
func (r *RingQP) InvMFormLvl(levelQ, levelP int, p, pOut PolyQP) {
	r.RingQ.InvMFormLvl(levelQ, p.Q, pOut.Q)
	r.RingP.InvMFormLvl(levelP, p.P, pOut.P)
}

// MulCoeffsMontgomeryLvl multiplies p1 by p2 coefficient-wise with a Montgomery modular reduction.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
func (r *RingQP) MulCoeffsMontgomeryLvl(levelQ, levelP int, p1, p2, p3 PolyQP) {
	r.RingQ.MulCoeffsMontgomeryLvl(levelQ, p1.Q, p2.Q, p3.Q)
	r.RingP.MulCoeffsMontgomeryLvl(levelP, p1.P, p2.P, p3.P)
}

// MulCoeffsMontgomeryConstantLvl multiplies p1 by p2 coefficient-wise with a constant-time Montgomery modular reduction.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
// Result is within [0, 2q-1].
func (r *RingQP) MulCoeffsMontgomeryConstantLvl(levelQ, levelP int, p1, p2, p3 PolyQP) {
	r.RingQ.MulCoeffsMontgomeryConstantLvl(levelQ, p1.Q, p2.Q, p3.Q)
	r.RingP.MulCoeffsMontgomeryConstantLvl(levelP, p1.P, p2.P, p3.P)
}

// MulCoeffsMontgomeryConstantAndAddNoModLvl multiplies p1 by p2 coefficient-wise with a
// constant-time Montgomery modular reduction and adds the result on p3.
// Result is within [0, 2q-1]
func (r *RingQP) MulCoeffsMontgomeryConstantAndAddNoModLvl(levelQ, levelP int, p1, p2, p3 PolyQP) {
	r.RingQ.MulCoeffsMontgomeryConstantAndAddNoModLvl(levelQ, p1.Q, p2.Q, p3.Q)
	r.RingP.MulCoeffsMontgomeryConstantAndAddNoModLvl(levelP, p1.P, p2.P, p3.P)
}

// MulCoeffsMontgomeryAndSubLvl multiplies p1 by p2 coefficient-wise with
// a Montgomery modular reduction and subtracts the result from p3.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
func (r *RingQP) MulCoeffsMontgomeryAndSubLvl(levelQ, levelP int, p1, p2, p3 PolyQP) {
	r.RingQ.MulCoeffsMontgomeryAndSubLvl(levelQ, p1.Q, p2.Q, p3.Q)
	r.RingP.MulCoeffsMontgomeryAndSubLvl(levelP, p1.P, p2.P, p3.P)
}

// MulCoeffsMontgomeryAndAddLvl multiplies p1 by p2 coefficient-wise with a
// Montgomery modular reduction and adds the result to p3.
// The operation is performed at levelQ for the ringQ and levelP for the ringP.
func (r *RingQP) MulCoeffsMontgomeryAndAddLvl(levelQ, levelP int, p1, p2, p3 PolyQP) {
	r.RingQ.MulCoeffsMontgomeryAndAddLvl(levelQ, p1.Q, p2.Q, p3.Q)
	r.RingP.MulCoeffsMontgomeryAndAddLvl(levelP, p1.P, p2.P, p3.P)
}

// ExtendBasisSmallNormAndCenter extends a small-norm polynomial polQ in R_Q to a polynomial
// polQP in R_QP.
func (r *RingQP) ExtendBasisSmallNormAndCenter(polyInQ *ring.Poly, levelP int, polyOutQ, polyOutP *ring.Poly) {
	var coeff, Q, QHalf, sign uint64
	Q = r.RingQ.Modulus[0]
	QHalf = Q >> 1

	if polyInQ != polyOutQ && polyOutQ != nil {
		polyOutQ.Copy(polyInQ)
	}

	for j := 0; j < r.RingQ.N; j++ {

		coeff = polyInQ.Coeffs[0][j]

		sign = 1
		if coeff > QHalf {
			coeff = Q - coeff
			sign = 0
		}

		for i, pi := range r.RingP.Modulus[:levelP+1] {
			polyOutP.Coeffs[i][j] = (coeff * sign) | (pi-coeff)*(sign^1)
		}
	}
}

// Copy copies the input polyQP on the target polyQP.
func (p *PolyQP) Copy(polFrom PolyQP) {
	p.Q.Copy(polFrom.Q)
	p.P.Copy(polFrom.P)
}

// GetDataLen returns the length in byte of the target PolyQP
func (p *PolyQP) GetDataLen(WithMetadata bool) (dataLen int) {
	return p.Q.GetDataLen(WithMetadata) + p.P.GetDataLen(WithMetadata)
}

// WriteTo writes a polyQP on the inpute data.
func (p *PolyQP) WriteTo(data []byte) (pt int, err error) {
	var inc int
	if inc, err = p.Q.WriteTo(data[pt:]); err != nil {
		return
	}
	pt += inc

	if inc, err = p.P.WriteTo(data[pt:]); err != nil {
		return
	}
	pt += inc

	return
}

// DecodePolyNew decodes the input bytes on the target polyQP.
func (p *PolyQP) DecodePolyNew(data []byte) (pt int, err error) {
	p.Q = new(ring.Poly)
	var inc int
	if inc, err = p.Q.DecodePolyNew(data[pt:]); err != nil {
		return
	}
	pt += inc

	p.P = new(ring.Poly)
	if inc, err = p.P.DecodePolyNew(data[pt:]); err != nil {
		return
	}
	pt += inc

	return
}