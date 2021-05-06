package mkrlwe

import (
	"github.com/ldsec/lattigo/v2/ring"
	"github.com/ldsec/lattigo/v2/rlwe"
)

// Relinearization implements the algorithm 3 in the appendix of the Chen paper
// It does relin directly by linearizing each entry of the extended ciphertext and stores it in cPrime (of size k+1)
// There are (k+1)**2 ciphertexts, and k pairs of (evaluation keys Di,bi)
func Relinearization(evaluationKeys []*MKEvaluationKey, publicKeys []*MKPublicKey, ct *MKCiphertext, params *rlwe.Parameters) {

	ringQ := GetRingQ(params)
	ringP := GetRingP(params)

	baseconverter := ring.NewFastBasisExtender(ringQ, ringP)
	level := ct.Ciphertexts.Level()

	k := uint64(len(evaluationKeys))
	restmpQ := make([]*ring.Poly, k+1)
	res := make([]*ring.Poly, k+1)
	restmpP := make([]*ring.Poly, k+1)

	for i := uint64(0); i < k+1; i++ {
		restmpQ[i] = ringQ.NewPoly()
		restmpP[i] = ringP.NewPoly()

		res[i] = ringQ.NewPoly()
	}

	cipherParts := ct.Ciphertexts.Value

	for i := uint64(1); i <= k; i++ {

		d0Q, d1Q, d2Q, d0P, d1P, d2P := prepareEvalKey(i, level, uint64(len(ringQ.Modulus)), params.Beta(), evaluationKeys)

		for j := uint64(1); j <= k; j++ {

			pkQ, pkP := preparePublicKey(j, level, uint64(len(ringQ.Modulus)), params.Beta(), publicKeys)

			decomposedIJQ, decomposedIJP := GInverse(cipherParts[i*(k+1)+j], params, level) // line 3

			cIJtmpQ := DotLvl(level, decomposedIJQ, pkQ, ringQ)
			cIJtmpP := Dot(decomposedIJP, pkP, ringP)

			cIJPrime := ringQ.NewPoly()

			baseconverter.ModDownSplitNTTPQ(level, cIJtmpQ, cIJtmpP, cIJPrime) // line 4

			decomposedTmpQ, decomposedTmpP := GInverse(cIJPrime, params, level) // inverse and matrix mult (line 5)

			tmpC0Q := DotLvl(level, decomposedTmpQ, d0Q, ringQ)
			tmpC0P := Dot(decomposedTmpP, d0P, ringP)

			tmpCiQ := DotLvl(level, decomposedTmpQ, d1Q, ringQ)
			tmpCiP := Dot(decomposedTmpP, d1P, ringP)

			ringQ.AddLvl(level, restmpQ[0], tmpC0Q, restmpQ[0])
			ringQ.AddLvl(level, restmpQ[i], tmpCiQ, restmpQ[i])

			ringP.Add(restmpP[0], tmpC0P, restmpP[0])
			ringP.Add(restmpP[i], tmpCiP, restmpP[i])

			tmpIJQ := DotLvl(level, decomposedIJQ, d2Q, ringQ) // line 6 of algorithm
			tmpIJP := Dot(decomposedIJP, d2P, ringP)

			ringQ.AddLvl(level, restmpQ[j], tmpIJQ, restmpQ[j])
			ringP.Add(restmpP[j], tmpIJP, restmpP[j])

		}
	}

	tmpModDown := ringQ.NewPoly()

	baseconverter.ModDownSplitNTTPQ(level, restmpQ[0], restmpP[0], tmpModDown)
	ringQ.AddLvl(level, cipherParts[0], tmpModDown, res[0])

	for i := uint64(1); i <= k; i++ {

		ringQ.AddLvl(level, cipherParts[i], cipherParts[(k+1)*i], res[i])

		baseconverter.ModDownSplitNTTPQ(level, restmpQ[i], restmpP[i], tmpModDown)
		ringQ.AddLvl(level, res[i], tmpModDown, res[i])

	}

	ct.Ciphertexts.SetValue(res)
}

// prepare evaluation key for operations in split crt basis
func prepareEvalKey(i, level, modulus, beta uint64, evaluationKeys []*MKEvaluationKey) (d0Q, d1Q, d2Q, d0P, d1P, d2P *MKDecomposedPoly) {

	di0 := evaluationKeys[i-1].Key[0]
	di1 := evaluationKeys[i-1].Key[1]
	di2 := evaluationKeys[i-1].Key[2]

	d0Q = new(MKDecomposedPoly)
	d0Q.Poly = make([]*ring.Poly, beta)
	d1Q = new(MKDecomposedPoly)
	d1Q.Poly = make([]*ring.Poly, beta)
	d2Q = new(MKDecomposedPoly)
	d2Q.Poly = make([]*ring.Poly, beta)
	d0P = new(MKDecomposedPoly)
	d0P.Poly = make([]*ring.Poly, beta)
	d1P = new(MKDecomposedPoly)
	d1P.Poly = make([]*ring.Poly, beta)
	d2P = new(MKDecomposedPoly)
	d2P.Poly = make([]*ring.Poly, beta)

	for u := uint64(0); u < beta; u++ {
		d0Q.Poly[u] = di0.Poly[u].CopyNew()
		d0Q.Poly[u].Coeffs = d0Q.Poly[u].Coeffs[:level+1]
		d1Q.Poly[u] = di1.Poly[u].CopyNew()
		d1Q.Poly[u].Coeffs = d1Q.Poly[u].Coeffs[:level+1]
		d2Q.Poly[u] = di2.Poly[u].CopyNew()
		d2Q.Poly[u].Coeffs = d2Q.Poly[u].Coeffs[:level+1]

		d0P.Poly[u] = di0.Poly[u].CopyNew()
		d0P.Poly[u].Coeffs = d0P.Poly[u].Coeffs[modulus:]
		d1P.Poly[u] = di1.Poly[u].CopyNew()
		d1P.Poly[u].Coeffs = d1P.Poly[u].Coeffs[modulus:]
		d2P.Poly[u] = di2.Poly[u].CopyNew()
		d2P.Poly[u].Coeffs = d2P.Poly[u].Coeffs[modulus:]
	}

	return
}

// prepare public key for operations in split crt basis
func preparePublicKey(j, level, modulus, beta uint64, publicKeys []*MKPublicKey) (pkQ, pkP *MKDecomposedPoly) {

	pkQ = new(MKDecomposedPoly)
	pkQ.Poly = make([]*ring.Poly, beta)
	pkP = new(MKDecomposedPoly)
	pkP.Poly = make([]*ring.Poly, beta)

	for u := uint64(0); u < beta; u++ {
		pkQ.Poly[u] = publicKeys[j-1].Key[0].Poly[u].CopyNew()
		pkQ.Poly[u].Coeffs = pkQ.Poly[u].Coeffs[:level+1]

		pkP.Poly[u] = publicKeys[j-1].Key[0].Poly[u].CopyNew()
		pkP.Poly[u].Coeffs = pkP.Poly[u].Coeffs[modulus:]

	}

	return
}

// GInverse is a method that returns the decomposition of a polynomial from R_qp to R_qp^beta
func GInverse(p *ring.Poly, params *rlwe.Parameters, level uint64) (*MKDecomposedPoly, *MKDecomposedPoly) {

	beta := params.Beta()
	ringQ := GetRingQ(params)
	ringP := GetRingP(params)

	resQ := new(MKDecomposedPoly)
	resP := new(MKDecomposedPoly)

	polynomialsQ := make([]*ring.Poly, beta)
	polynomialsP := make([]*ring.Poly, beta)

	invPoly := ringQ.NewPoly()
	ringQ.InvNTTLvl(level, p, invPoly)

	// generate each poly decomposed in the base
	for i := uint64(0); i < beta; i++ {

		polynomialsQ[i] = ringQ.NewPoly()
		polynomialsP[i] = ringP.NewPoly()

		decomposeAndSplitNTT(level, i, p, invPoly, polynomialsQ[i], polynomialsP[i], params, ringQ, ringP)

	}

	resQ.Poly = polynomialsQ
	resP.Poly = polynomialsP

	return resQ, resP
}

// decomposeAndSplitNTT decomposes the input polynomial into the target CRT basis.
func decomposeAndSplitNTT(level, beta uint64, c2NTT, c2InvNTT, c2QiQ, c2QiP *ring.Poly, params *rlwe.Parameters, ringQ, ringP *ring.Ring) {

	decomposer := ring.NewDecomposer(ringQ.Modulus, ringP.Modulus)

	decomposer.DecomposeAndSplit(level, beta, c2InvNTT, c2QiQ, c2QiP)

	p0idxst := beta * params.Alpha()
	p0idxed := p0idxst + decomposer.Xalpha()[beta]

	// c2_qi = cx mod qi mod qi
	for x := uint64(0); x < level+1; x++ {

		qi := ringQ.Modulus[x]
		nttPsi := ringQ.GetNttPsi()[x]
		bredParams := ringQ.GetBredParams()[x]
		mredParams := ringQ.GetMredParams()[x]

		if p0idxst <= x && x < p0idxed {
			p0tmp := c2NTT.Coeffs[x]
			p1tmp := c2QiQ.Coeffs[x]
			for j := uint64(0); j < ringQ.N; j++ {
				p1tmp[j] = p0tmp[j]
			}
		} else {
			ring.NTTLazy(c2QiQ.Coeffs[x], c2QiQ.Coeffs[x], ringQ.N, nttPsi, qi, mredParams, bredParams)
		}
	}
	// c2QiP = c2 mod qi mod pj
	ringP.NTTLazy(c2QiP, c2QiP)
}
