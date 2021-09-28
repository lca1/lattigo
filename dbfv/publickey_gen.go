//Package dbfv implements a distributed (or threshold) version of the BFV scheme that enables secure multiparty computation solutions with secret-shared secret keys.
package dbfv

import (
	"github.com/ldsec/lattigo/bfv"
	"github.com/ldsec/lattigo/ring"
)

// CKGProtocol is the structure storing the parameters and state for a party in the collective key generation protocol.
type CKGProtocol struct {
	context         *ring.Context
	gaussianSampler *ring.KYSampler
}

// CKGShare is a struct holding a CKG share.
type CKGShare struct {
	*ring.Poly
}

// UnmarshalBinary decode a marshaled CKG share on the target CKG share.
func (share *CKGShare) UnmarshalBinary(data []byte) error {
	if share.Poly == nil {
		share.Poly = new(ring.Poly)
	}
	err := share.Poly.UnmarshalBinary(data)
	return err

}

// NewCKGProtocol creates a new CKGProtocol instance
func NewCKGProtocol(params *bfv.Parameters) *CKGProtocol {

	if !params.IsValid() {
		panic("cannot NewCKGProtocol : params not valid (check if they where generated properly)")
	}

	context := newDbfvContext(params)
	ckg := new(CKGProtocol)
	ckg.context = context.contextQP
	ckg.gaussianSampler = context.gaussianSampler
	return ckg
}

// AllocateShares allocates the CKG shares.
func (ckg *CKGProtocol) AllocateShares() CKGShare {
	return CKGShare{ckg.context.NewPoly()}
}

// GenShare generates the party's public key share from its secret key as:
//
// crs*s_i + e_i
//
// for the receiver protocol. Has no effect is the share was already generated.
func (ckg *CKGProtocol) GenShare(sk *ring.Poly, crs *ring.Poly, shareOut CKGShare) {
	ckg.gaussianSampler.SampleNTT(shareOut.Poly)
	ckg.context.MulCoeffsMontgomeryAndSub(sk, crs, shareOut.Poly)
}

// AggregateShares aggregates a new share to the aggregate key
func (ckg *CKGProtocol) AggregateShares(share1, share2, shareOut CKGShare) {
	ckg.context.Add(share1.Poly, share2.Poly, shareOut.Poly)
}

// GenPublicKey return the current aggregation of the received shares as a bfv.PublicKey.
func (ckg *CKGProtocol) GenPublicKey(roundShare CKGShare, crs *ring.Poly, pubkey *bfv.PublicKey) {
	pubkey.Set([2]*ring.Poly{roundShare.Poly, crs})
}
