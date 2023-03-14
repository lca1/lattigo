package rlwe

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"github.com/tuneinsight/lattigo/v4/utils/bignum/polynomial"
	"github.com/tuneinsight/lattigo/v4/utils/buffer"
	"github.com/tuneinsight/lattigo/v4/utils/structs"
)

// PowerBasis is a struct storing powers of a ciphertext.
type PowerBasis struct {
	polynomial.Basis
	Value structs.Map[int, Ciphertext]
}

// NewPowerBasis creates a new PowerBasis. It takes as input a ciphertext
// and a basistype. The struct treats the input ciphertext as a monomial X and
// can be used to generates power of this monomial X^{n} in the given BasisType.
func NewPowerBasis(ct *Ciphertext, basis polynomial.Basis) (p *PowerBasis) {
	p = new(PowerBasis)
	p.Value = make(map[int]*Ciphertext)
	p.Value[1] = ct.CopyNew()
	p.Basis = basis
	return
}

// MarshalBinary encodes the object into a binary form on a newly allocated slice of bytes.
func (p *PowerBasis) MarshalBinary() (data []byte, err error) {
	buf := bytes.NewBuffer([]byte{})
	_, err = p.WriteTo(buf)
	return buf.Bytes(), err
}

// UnmarshalBinary decodes a slice of bytes generated by
// MarshalBinary or WriteTo on the object.
func (p *PowerBasis) UnmarshalBinary(data []byte) (err error) {
	_, err = p.ReadFrom(bytes.NewBuffer(data))
	return
}

// WriteTo writes the object on an io.Writer.
// To ensure optimal efficiency and minimal allocations, the user is encouraged
// to provide a struct implementing the interface buffer.Writer, which defines
// a subset of the method of the bufio.Writer.
// If w is not compliant to the buffer.Writer interface, it will be wrapped in
// a new bufio.Writer.
// For additional information, see lattigo/utils/buffer/writer.go.
func (p *PowerBasis) WriteTo(w io.Writer) (n int64, err error) {

	switch w := w.(type) {
	case buffer.Writer:

		var inc1 int

		if inc1, err = buffer.WriteUint8(w, uint8(p.Basis)); err != nil {
			return n + int64(inc1), err
		}

		n += int64(inc1)

		inc2, err := p.Value.WriteTo(w)

		return n + inc2, err

	default:
		return p.WriteTo(bufio.NewWriter(w))
	}
}

// ReadFrom reads on the object from an io.Writer.
// To ensure optimal efficiency and minimal allocations, the user is encouraged
// to provide a struct implementing the interface buffer.Reader, which defines
// a subset of the method of the bufio.Reader.
// If r is not compliant to the buffer.Reader interface, it will be wrapped in
// a new bufio.Reader.
// For additional information, see lattigo/utils/buffer/reader.go.
func (p *PowerBasis) ReadFrom(r io.Reader) (n int64, err error) {
	switch r := r.(type) {
	case buffer.Reader:
		var inc1 int

		var Basis uint8

		if inc1, err = buffer.ReadUint8(r, &Basis); err != nil {
			return n + int64(inc1), err
		}

		n += int64(inc1)

		p.Basis = polynomial.Basis(Basis)

		if p.Value == nil {
			p.Value = map[int]*Ciphertext{}
		}

		inc2, err := p.Value.ReadFrom(r)

		return n + inc2, err

	default:
		return p.ReadFrom(bufio.NewReader(r))
	}
}

// BinarySize returns the size in bytes of the object
// when encoded using Encode.
func (p *PowerBasis) BinarySize() (size int) {
	return 1 + p.Value.BinarySize()
}

// Encode encodes the object into a binary form on a preallocated slice of bytes
// and returns the number of bytes written.
func (p *PowerBasis) Encode(data []byte) (n int, err error) {

	if len(data) < p.BinarySize() {
		return n, fmt.Errorf("cannot Encode: len(data)=%d < %d", len(data), p.BinarySize())
	}

	data[n] = uint8(p.Basis)
	n++

	inc, err := p.Value.Encode(data[n:])

	return n + inc, err
}

// Decode decodes a slice of bytes generated by Encode
// on the object and returns the number of bytes read.
func (p *PowerBasis) Decode(data []byte) (n int, err error) {

	p.Basis = polynomial.Basis(data[n])
	n++

	if p.Value == nil {
		p.Value = map[int]*Ciphertext{}
	}

	inc, err := p.Value.Decode(data[n:])

	return n + inc, err
}