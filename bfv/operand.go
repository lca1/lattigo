package bfv

import (
	"github.com/ldsec/lattigo/v2/ring"
	"github.com/ldsec/lattigo/v2/utils"
)

// Operand is a common interface for Ciphertext and Plaintext.
type Operand interface {
	El() *Element
	Degree() uint64
}

// Element is a common struct for Plaintexts and Ciphertexts. It stores a value
// as a slice of polynomials, and an isNTT flag that indicates if the element is in the NTT domain.
type Element struct {
	value []*ring.Poly
}

func newEleCT(params *Parameters, degree uint64) *Element {
	el := new(Element)
	el.value = make([]*ring.Poly, degree+1)
	for i := uint64(0); i < degree+1; i++ {
		el.value[i] = ring.NewPoly(params.N(), params.QiCount())
	}
	return el
}

func newElePTZQ(params *Parameters) *Element {
	el := new(Element)
	el.value = []*ring.Poly{ring.NewPoly(params.N(), params.QiCount())}
	return el
}

func newElePTZT(params *Parameters) *Element {
	el := new(Element)
	el.value = []*ring.Poly{ring.NewPoly(params.N(), 1)}
	return el
}

func newElePTMul(params *Parameters) *Element {
	el := new(Element)
	el.value = []*ring.Poly{ring.NewPoly(params.N(), params.QiCount())}
	return el
}

// NewElementRandom creates a new Element with random coefficients
func populateElementRandom(prng utils.PRNG, params *Parameters, el *Element) {

	ringQ, err := ring.NewRing(params.N(), params.qi)
	if err != nil {
		panic(err)
	}
	sampler := ring.NewUniformSampler(prng, ringQ)
	for i := range el.value {
		sampler.Read(el.value[i])
	}
}

// Value returns the value of the target Element (as a slice of polynomials in CRT form).
func (el *Element) Value() []*ring.Poly {
	return el.value
}

// SetValue assigns the input slice of polynomials to the target Element value.
func (el *Element) SetValue(value []*ring.Poly) {
	el.value = value
}

// Degree returns the degree of the target Element.
func (el *Element) Degree() uint64 {
	return uint64(len(el.value) - 1)
}

// Resize resizes the target Element degree to the degree given as input. If the input degree is bigger, then
// it will append new empty polynomials; if the degree is smaller, it will delete polynomials until the degree matches
// the input degree.
func (el *Element) Resize(params *Parameters, degree uint64) {
	if el.Degree() > degree {
		el.value = el.value[:degree+1]
	} else if el.Degree() < degree {
		for el.Degree() < degree {
			el.value = append(el.value, []*ring.Poly{new(ring.Poly)}...)
			el.value[el.Degree()].Coeffs = make([][]uint64, len(params.qi))
			for i := 0; i < len(params.qi); i++ {
				el.value[el.Degree()].Coeffs[i] = make([]uint64, uint64(1<<params.logN))
			}
		}
	}
}

// CopyNew creates a new Element which is a copy of the target Element, and returns the value as
// a Element.
func (el *Element) CopyNew() *Element {

	ctxCopy := new(Element)

	ctxCopy.value = make([]*ring.Poly, el.Degree()+1)
	for i := range el.value {
		ctxCopy.value[i] = el.value[i].CopyNew()
	}

	return ctxCopy
}

// Copy copies the value and parameters of the input on the target Element.
func (el *Element) Copy(ctxCopy *Element) {
	if el != ctxCopy {
		for i := range ctxCopy.Value() {
			el.Value()[i].Copy(ctxCopy.Value()[i])
		}
	}
}

// El sets the target Element type to Element.
func (el *Element) El() *Element {
	return el
}

// Ciphertext sets the target Element type to Ciphertext.
func (el *Element) Ciphertext() *Ciphertext {
	return &Ciphertext{el}
}

// Plaintext sets the target Element type to Plaintext.
func (el *Element) Plaintext() *Plaintext {
	return &Plaintext{el, el.value[0]}
}
