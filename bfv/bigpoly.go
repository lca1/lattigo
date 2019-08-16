package bfv

import (
	"github.com/lca1/lattigo/ring"
)

type BigPoly struct {
	value      []*ring.Poly
	bfvcontext *BfvContext
	isNTT      bool
}

type BfvElement interface {
	Value() []*ring.Poly
	SetValue([]*ring.Poly)
	BfvContext() *BfvContext
	Resize(uint64)
	CopyNew() BfvElement
	Copy(BfvElement) error
	Degree() uint64
	NTT(BfvElement) error
	InvNTT(BfvElement) error
	IsNTT() bool
	SetIsNTT(bool)
}
