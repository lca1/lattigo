package ckks

import (
	"github.com/ldsec/lattigo/v2/ring"
	"github.com/ldsec/lattigo/v2/rlwe"
	"github.com/ldsec/lattigo/v2/utils"
	"runtime"
)

// Trace maps X -> sum((-1)^i * X^{i*n+1}) for 0 <= i < N
// For log(n) = logSlotStart and log(N/2) = logSlotsEnd
func (eval *evaluator) Trace(ctIn *Ciphertext, logSlotsStart, logSlotsEnd int, ctOut *Ciphertext) {
	if ctIn != ctOut {
		ctOut.Copy(ctIn)
	} else {
		ctOut = ctIn
	}
	for i := logSlotsStart; i < logSlotsEnd; i++ {
		eval.permuteNTT(ctOut, eval.params.GaloisElementForColumnRotationBy(1<<i), eval.ctxpool)
		ctPool := &Ciphertext{Ciphertext: eval.ctxpool.Ciphertext, Scale: ctOut.Scale}
		ctPool.Value = ctPool.Value[:2]
		eval.Add(ctOut, ctPool, ctOut)
	}
}

// TraceNew maps X -> sum((-1)^i * X^{i*n+1}) for 0 <= i < N and returns the result on a new ciphertext.
// For log(n) = logSlotStart and log(N/2) = logSlotsEnd
func (eval *evaluator) TraceNew(ctIn *Ciphertext, logSlotsStart, logSlotsEnd int) (ctOut *Ciphertext) {
	ctOut = NewCiphertext(eval.params, 1, ctIn.Level(), ctIn.Scale)
	eval.Trace(ctIn, logSlotsStart, logSlotsEnd, ctOut)
	return
}

// RotateHoistedNew takes an input Ciphertext and a list of rotations and returns a map of Ciphertext, where each element of the map is the input Ciphertext
// rotation by one element of the list. It is much faster than sequential calls to Rotate.
func (eval *evaluator) RotateHoistedNew(ctIn *Ciphertext, rotations []int) (ctOut map[int]*Ciphertext) {
	ctOut = make(map[int]*Ciphertext)
	for _, i := range rotations {
		ctOut[i] = NewCiphertext(eval.params, 1, ctIn.Level(), ctIn.Scale)
	}
	eval.RotateHoisted(ctIn, rotations, ctOut)
	return
}

// RotateHoisted takes an input Ciphertext and a list of rotations and populates a map of pre-allocated Ciphertexts,
// where each element of the map is the input Ciphertext rotation by one element of the list.
// It is much faster than sequential calls to Rotate.
func (eval *evaluator) RotateHoisted(ctIn *Ciphertext, rotations []int, ctOut map[int]*Ciphertext) {
	levelQ := ctIn.Level()
	eval.DecomposeNTT(levelQ, eval.params.PCount()-1, eval.params.PCount(), ctIn.Value[1], eval.PoolDecompQP)
	for _, i := range rotations {
		if i == 0 {
			ctOut[i].Copy(ctIn)
		} else {
			eval.PermuteNTTHoisted(levelQ, ctIn.Value[0], ctIn.Value[1], eval.PoolDecompQP, i, ctOut[i].Value[0], ctOut[i].Value[1])
		}
	}
}

// LinearTransformNew evaluates a linear transform on the ciphertext and returns the result on a new ciphertext.
// The linearTransform can either be an (ordered) list of PtDiagMatrix or a single PtDiagMatrix.
// In either case a list of ciphertext is return (the second case returnign a list of
// containing a single ciphertext. A PtDiagMatrix is a diagonalized plaintext matrix contructed with an Encoder using
// the method encoder.EncodeDiagMatrixAtLvl(*).
func (eval *evaluator) LinearTransformNew(ctIn *Ciphertext, linearTransform interface{}) (ctOut []*Ciphertext) {

	switch element := linearTransform.(type) {
	case []PtDiagMatrix:
		ctOut = make([]*Ciphertext, len(element))

		var maxLevel int
		for _, matrix := range element {
			maxLevel = utils.MaxInt(maxLevel, matrix.Level)
		}

		minLevel := utils.MinInt(maxLevel, ctIn.Level())

		eval.DecomposeNTT(minLevel, eval.params.PCount()-1, eval.params.PCount(), ctIn.Value[1], eval.PoolDecompQP)

		for i, matrix := range element {
			ctOut[i] = NewCiphertext(eval.params, 1, minLevel, ctIn.Scale)

			if matrix.Naive {
				eval.MultiplyByDiagMatrix(ctIn, matrix, eval.PoolDecompQP, ctOut[i])
			} else {
				eval.MultiplyByDiagMatrixBSGS(ctIn, matrix, eval.PoolDecompQP, ctOut[i])
			}
		}

	case PtDiagMatrix:

		minLevel := utils.MinInt(element.Level, ctIn.Level())
		eval.DecomposeNTT(minLevel, eval.params.PCount()-1, eval.params.PCount(), ctIn.Value[1], eval.PoolDecompQP)

		ctOut = []*Ciphertext{NewCiphertext(eval.params, 1, minLevel, ctIn.Scale)}

		if element.Naive {
			eval.MultiplyByDiagMatrix(ctIn, element, eval.PoolDecompQP, ctOut[0])
		} else {
			eval.MultiplyByDiagMatrixBSGS(ctIn, element, eval.PoolDecompQP, ctOut[0])
		}
	}
	return
}

// LinearTransformNew evaluates a linear transform on the pre-allocated ciphertexts.
// The linearTransform can either be an (ordered) list of PtDiagMatrix or a single PtDiagMatrix.
// In either case a list of ciphertext is return (the second case returnign a list of
// containing a single ciphertext. A PtDiagMatrix is a diagonalized plaintext matrix contructed with an Encoder using
// the method encoder.EncodeDiagMatrixAtLvl(*).
func (eval *evaluator) LinearTransform(ctIn *Ciphertext, linearTransform interface{}, ctOut []*Ciphertext) {

	switch element := linearTransform.(type) {
	case []PtDiagMatrix:
		var maxLevel int
		for _, matrix := range element {
			maxLevel = utils.MaxInt(maxLevel, matrix.Level)
		}

		minLevel := utils.MinInt(maxLevel, ctIn.Level())

		eval.DecomposeNTT(minLevel, eval.params.PCount()-1, eval.params.PCount(), ctIn.Value[1], eval.PoolDecompQP)

		for i, matrix := range element {
			if matrix.Naive {
				eval.MultiplyByDiagMatrix(ctIn, matrix, eval.PoolDecompQP, ctOut[i])
			} else {
				eval.MultiplyByDiagMatrixBSGS(ctIn, matrix, eval.PoolDecompQP, ctOut[i])
			}
		}

	case PtDiagMatrix:
		minLevel := utils.MinInt(element.Level, ctIn.Level())
		eval.DecomposeNTT(minLevel, eval.params.PCount()-1, eval.params.PCount(), ctIn.Value[1], eval.PoolDecompQP)
		if element.Naive {
			eval.MultiplyByDiagMatrix(ctIn, element, eval.PoolDecompQP, ctOut[0])
		} else {
			eval.MultiplyByDiagMatrixBSGS(ctIn, element, eval.PoolDecompQP, ctOut[0])
		}
	}
}

// InnerSumLog applies an optimized inner sum on the ciphetext (log2(n) + HW(n) rotations with double hoisting).
// The operation assumes that `ctIn` encrypts SlotCount/`batchSize` sub-vectors of size `batchSize` which it adds together (in parallel) by groups of `n`.
// It outputs in ctOut a ciphertext for which the "leftmost" sub-vector of each group is equal to the sum of the group.
// This method is faster than InnerSum when the number of rotations is large and uses log2(n) + HW(n) insteadn of 'n' keys.
func (eval *evaluator) InnerSumLog(ctIn *Ciphertext, batchSize, n int, ctOut *Ciphertext) {

	ringQ := eval.params.RingQ()
	ringP := eval.params.RingP()
	ringQP := rlwe.RingQP{RingQ: ringQ, RingP: ringP}

	levelQ := ctIn.Level()
	levelP := len(ringP.Modulus) - 1

	//QiOverF := eval.params.QiOverflowMargin(levelQ)
	//PiOverF := eval.params.PiOverflowMargin()

	if n == 1 {
		if ctIn != ctOut {
			ring.CopyValuesLvl(levelQ, ctIn.Value[0], ctOut.Value[0])
			ring.CopyValuesLvl(levelQ, ctIn.Value[1], ctOut.Value[1])
		}
	} else {

		// Memory pool for ctIn = ctIn + rot(ctIn, 2^i) in Q
		tmpc0 := eval.poolQMul[0] // unused memory pool from evaluator
		tmpc1 := eval.poolQMul[1] // unused memory pool from evaluator

		tmpc1.IsNTT = true

		c0OutQP := eval.Pool[2]
		c1OutQP := eval.Pool[3]
		c0QP := eval.Pool[4]
		c1QP := eval.Pool[5]

		state := false
		copy := true
		// Binary reading of the input n
		for i, j := 0, n; j > 0; i, j = i+1, j>>1 {

			// Starts by decomposing the input ciphertext
			if i == 0 {
				// If first iteration, then copies directly from the input ciphertext that hasn't been rotated
				eval.DecomposeNTT(levelQ, levelP, levelP+1, ctIn.Value[1], eval.PoolDecompQP)
			} else {
				// Else copies from the rotated input ciphertext
				eval.DecomposeNTT(levelQ, levelP, levelP+1, tmpc1, eval.PoolDecompQP)
			}

			// If the binary reading scans a 1
			if j&1 == 1 {

				k := n - (n & ((2 << i) - 1))
				k *= batchSize

				// If the rotation is not zero
				if k != 0 {

					// Rotate((tmpc0, tmpc1), k)
					if i == 0 {
						eval.PermuteNTTHoistedNoModDown(levelQ, ctIn.Value[0], eval.PoolDecompQP, k, c0QP.Q, c1QP.Q, c0QP.P, c1QP.P)
					} else {
						eval.PermuteNTTHoistedNoModDown(levelQ, tmpc0, eval.PoolDecompQP, k, c0QP.Q, c1QP.Q, c0QP.P, c1QP.P)
					}

					// ctOut += Rotate((tmpc0, tmpc1), k)
					if copy {
						ringQP.CopyValuesLvl(levelQ, levelP, c0QP, c0OutQP)
						ringQP.CopyValuesLvl(levelQ, levelP, c1QP, c1OutQP)
						copy = false
					} else {
						ringQP.AddLvl(levelQ, levelP, c0OutQP, c0QP, c0OutQP)
						ringQP.AddLvl(levelQ, levelP, c1OutQP, c1QP, c1OutQP)
					}
				} else {

					state = true

					// if n is not a power of two
					if n&(n-1) != 0 {
						eval.Baseconverter.ModDownQPtoQNTT(levelQ, levelP, c0OutQP.Q, c0OutQP.P, c0OutQP.Q) // Division by P
						eval.Baseconverter.ModDownQPtoQNTT(levelQ, levelP, c1OutQP.Q, c1OutQP.P, c1OutQP.Q) // Division by P

						// ctOut += (tmpc0, tmpc1)
						ringQ.AddLvl(levelQ, c0OutQP.Q, tmpc0, ctOut.Value[0])
						ringQ.AddLvl(levelQ, c1OutQP.Q, tmpc1, ctOut.Value[1])

					} else {
						ring.CopyValuesLvl(levelQ, tmpc0, ctOut.Value[0])
						ring.CopyValuesLvl(levelQ, tmpc1, ctOut.Value[1])
						ctOut.Value[0].IsNTT = true
						ctOut.Value[1].IsNTT = true
					}
				}
			}

			if !state {
				if i == 0 {
					eval.PermuteNTTHoisted(levelQ, ctIn.Value[0], ctIn.Value[1], eval.PoolDecompQP, (1<<i)*batchSize, tmpc0, tmpc1)
					ringQ.AddLvl(levelQ, tmpc0, ctIn.Value[0], tmpc0)
					ringQ.AddLvl(levelQ, tmpc1, ctIn.Value[1], tmpc1)
				} else {
					// (tmpc0, tmpc1) = Rotate((tmpc0, tmpc1), 2^i)
					eval.PermuteNTTHoisted(levelQ, tmpc0, tmpc1, eval.PoolDecompQP, (1<<i)*batchSize, c0QP.Q, c1QP.Q)
					ringQ.AddLvl(levelQ, tmpc0, c0QP.Q, tmpc0)
					ringQ.AddLvl(levelQ, tmpc1, c1QP.Q, tmpc1)
				}
			}
		}
	}
}

// InnerSum applies an naive inner sum on the ciphetext (n rotations with single hoisting).
// The operation assumes that `ctIn` encrypts SlotCount/`batchSize` sub-vectors of size `batchSize` which it adds together (in parallel) by groups of `n`.
// It outputs in ctOut a ciphertext for which the "leftmost" sub-vector of each group is equal to the sum of the group.
// This method is faster than InnerSumLog when the number of rotations is small but uses 'n' keys instead of log(n) + HW(n).
func (eval *evaluator) InnerSum(ctIn *Ciphertext, batchSize, n int, ctOut *Ciphertext) {

	ringQ := eval.params.RingQ()
	ringP := eval.params.RingP()
	ringQP := rlwe.RingQP{RingQ: ringQ, RingP: ringP}

	levelQ := ctIn.Level()
	levelP := len(ringP.Modulus) - 1

	QiOverF := eval.params.QiOverflowMargin(levelQ) >> 1
	PiOverF := eval.params.PiOverflowMargin(levelP) >> 1

	// If sum with only the first element, then returns the input
	if n == 1 {

		// If the input-output points differ, copies on the output
		if ctIn != ctOut {
			ring.CopyValuesLvl(levelQ, ctIn.Value[0], ctOut.Value[0])
			ring.CopyValuesLvl(levelQ, ctIn.Value[1], ctOut.Value[1])
		}
		// If sum on at least two elements
	} else {

		// List of n-2 rotations
		rotations := []int{}
		for i := 1; i < n; i++ {
			rotations = append(rotations, i*batchSize)
		}

		// Memory pool
		tmp0QP := eval.Pool[1]
		tmp1QP := eval.Pool[2]

		// Basis decomposition
		eval.DecomposeNTT(levelQ, levelP, levelP+1, ctIn.Value[1], eval.PoolDecompQP)

		// Pre-rotates all [1, ..., n-1] rotations
		// Hoisted rotation without division by P
		ctInRotQP := eval.RotateHoistedNoModDownNew(levelQ, rotations, ctIn.Value[0], eval.PoolDecompQP)

		// P*c0 -> tmp0QP.Q
		ringQ.MulScalarBigintLvl(levelQ, ctIn.Value[0], ringP.ModulusBigint, tmp0QP.Q)

		// Adds phi_k(P*c0) on each of the vecRotQ
		// Does not need to add on the vecRotP because mod P === 0
		var reduce int
		// Sums elements [2, ..., n-1]
		for i := 1; i < n; i++ {

			j := i * batchSize

			if i == 1 {
				ringQP.CopyValuesLvl(levelQ, levelP, ctInRotQP[j][0], tmp0QP)
				ringQP.CopyValuesLvl(levelQ, levelP, ctInRotQP[j][1], tmp1QP)
			} else {
				ringQP.AddNoModLvl(levelQ, levelP, tmp0QP, ctInRotQP[j][0], tmp0QP)
				ringQP.AddNoModLvl(levelQ, levelP, tmp1QP, ctInRotQP[j][1], tmp1QP)
			}

			if reduce%QiOverF == QiOverF-1 {
				ringQ.ReduceLvl(levelQ, tmp0QP.Q, tmp0QP.Q)
				ringQ.ReduceLvl(levelQ, tmp1QP.Q, tmp1QP.Q)
			}

			if reduce%PiOverF == PiOverF-1 {
				ringP.ReduceLvl(levelP, tmp0QP.P, tmp0QP.P)
				ringP.ReduceLvl(levelP, tmp1QP.P, tmp1QP.P)
			}

			reduce++
		}

		if reduce%QiOverF != 0 {
			ringQ.ReduceLvl(levelQ, tmp0QP.Q, tmp0QP.Q)
			ringQ.ReduceLvl(levelQ, tmp1QP.Q, tmp1QP.Q)
		}

		if reduce%PiOverF != 0 {
			ringP.ReduceLvl(levelP, tmp0QP.P, tmp0QP.P)
			ringP.ReduceLvl(levelP, tmp1QP.P, tmp1QP.P)
		}

		// Division by P of sum(elements [2, ..., n-1] )
		eval.Baseconverter.ModDownQPtoQNTT(levelQ, levelP, tmp0QP.Q, tmp0QP.P, tmp0QP.Q) // sum_{i=1, n-1}(phi(d0))/P
		eval.Baseconverter.ModDownQPtoQNTT(levelQ, levelP, tmp1QP.Q, tmp1QP.P, tmp1QP.Q) // sum_{i=1, n-1}(phi(d1))/P

		// Adds element[1] (which did not require rotation)
		ringQ.AddLvl(levelQ, ctIn.Value[0], tmp0QP.Q, ctOut.Value[0]) // sum_{i=1, n-1}(phi(d0))/P + ct0
		ringQ.AddLvl(levelQ, ctIn.Value[1], tmp1QP.Q, ctOut.Value[1]) // sum_{i=1, n-1}(phi(d1))/P + ct1
	}
}

// ReplicateLog applies an optimized replication on the ciphetext (log2(n) + HW(n) rotations with double hoisting).
// It acts as the inverse of a inner sum (summing elements from left to right).
// The replication is parameterized by the size of the sub-vectors to replicate "batchSize" and
// the number of time "n" they need to be replicated.
// To ensure correctness, a gap of zero values of size batchSize * (n-1) must exist between
// two consecutive sub-vectors to replicate.
// This method is faster than Replicate when the number of rotations is large and uses log2(n) + HW(n) instead of 'n'.
func (eval *evaluator) ReplicateLog(ctIn *Ciphertext, batchSize, n int, ctOut *Ciphertext) {
	eval.InnerSumLog(ctIn, -batchSize, n, ctOut)
}

// Replicate applies naive replication on the ciphetext (n rotations with single hoisting).
// It acts as the inverse of a inner sum (summing elements from left to right).
// The replication is parameterized by the size of the sub-vectors to replicate "batchSize" and
// the number of time "n" they need to be replicated.
// To ensure correctness, a gap of zero values of size batchSize * (n-1) must exist between
// two consecutive sub-vectors to replicate.
// This method is faster than ReplicateLog when the number of rotations is small but uses 'n' keys instead of log2(n) + HW(n).
func (eval *evaluator) Replicate(ctIn *Ciphertext, batchSize, n int, ctOut *Ciphertext) {
	eval.InnerSum(ctIn, -batchSize, n, ctOut)
}

// MultiplyByDiagMatrix multiplies the ciphertext "ctIn" by the plaintext matrix "matrix" and returns the result on the ciphertext
// "ctOut". Memory pools for the decomposed ciphertext PoolDecompQ, PoolDecompP must be provided, those are list of poly of ringQ and ringP
// respectively, each of size params.Beta().
// The naive approach is used (single hoisting and no baby-step giant-step), which is faster than MultiplyByDiagMatrixBSGS
// for matrix of only a few non-zero diagonals but uses more keys.
func (eval *evaluator) MultiplyByDiagMatrix(ctIn *Ciphertext, matrix PtDiagMatrix, PoolDecompQP []rlwe.PolyQP, ctOut *Ciphertext) {

	ringQ := eval.params.RingQ()
	ringP := eval.params.RingP()
	ringQP := rlwe.RingQP{RingQ: ringQ, RingP: ringP}

	levelQ := utils.MinInt(ctOut.Level(), utils.MinInt(ctIn.Level(), matrix.Level))
	levelP := len(ringP.Modulus) - 1

	QiOverF := eval.params.QiOverflowMargin(levelQ)
	PiOverF := eval.params.PiOverflowMargin(levelP)

	c0OutQP := rlwe.PolyQP{Q: ctOut.Value[0], P: eval.Pool[5].Q}
	c1OutQP := rlwe.PolyQP{Q: ctOut.Value[1], P: eval.Pool[5].P}

	ct0TimesP := eval.Pool[0].Q // ct0 * P mod Q
	tmp0QP := eval.Pool[1]
	tmp1QP := eval.Pool[2]
	ksRes0QP := eval.Pool[3]
	ksRes1QP := eval.Pool[4]

	ring.CopyValuesLvl(levelQ, ctIn.Value[0], eval.ctxpool.Value[0])
	ring.CopyValuesLvl(levelQ, ctIn.Value[1], eval.ctxpool.Value[1])
	ctInTmp0, ctInTmp1 := eval.ctxpool.Value[0], eval.ctxpool.Value[1]

	ringQ.MulScalarBigintLvl(levelQ, ctInTmp0, ringP.ModulusBigint, ct0TimesP) // P*c0

	var state bool
	var cnt int
	for k := range matrix.Vec {

		k &= int((ringQ.N >> 1) - 1)

		if k == 0 {
			state = true
		} else {

			galEl := eval.params.GaloisElementForColumnRotationBy(k)

			rtk, generated := eval.rtks.Keys[galEl]
			if !generated {
				panic("switching key not available")
			}

			index := eval.permuteNTTIndex[galEl]

			eval.KeyswitchHoistedNoModDown(levelQ, PoolDecompQP, rtk, ksRes0QP.Q, ksRes1QP.Q, ksRes0QP.P, ksRes1QP.P)
			ringQ.AddLvl(levelQ, ksRes0QP.Q, ct0TimesP, ksRes0QP.Q)
			ringQP.PermuteNTTWithIndexLvl(levelQ, levelP, ksRes0QP, index, tmp0QP)
			ringQP.PermuteNTTWithIndexLvl(levelQ, levelP, ksRes1QP, index, tmp1QP)

			if cnt == 0 {
				// keyswitch(c1_Q) = (d0_QP, d1_QP)
				ringQP.MulCoeffsMontgomeryLvl(levelQ, levelP, matrix.Vec[k], tmp0QP, c0OutQP)
				ringQP.MulCoeffsMontgomeryLvl(levelQ, levelP, matrix.Vec[k], tmp1QP, c1OutQP)
			} else {
				// keyswitch(c1_Q) = (d0_QP, d1_QP)
				ringQP.MulCoeffsMontgomeryAndAddLvl(levelQ, levelP, matrix.Vec[k], tmp0QP, c0OutQP)
				ringQP.MulCoeffsMontgomeryAndAddLvl(levelQ, levelP, matrix.Vec[k], tmp1QP, c1OutQP)
			}

			if cnt%QiOverF == QiOverF-1 {
				ringQ.ReduceLvl(levelQ, c0OutQP.Q, c0OutQP.Q)
				ringQ.ReduceLvl(levelQ, c1OutQP.Q, c1OutQP.Q)
			}

			if cnt%PiOverF == PiOverF-1 {
				ringP.ReduceLvl(levelP, c0OutQP.P, c0OutQP.P)
				ringP.ReduceLvl(levelP, c1OutQP.P, c1OutQP.P)
			}

			cnt++
		}
	}

	if cnt%QiOverF == 0 {
		ringQ.ReduceLvl(levelQ, c0OutQP.Q, c0OutQP.Q)
		ringQ.ReduceLvl(levelQ, c1OutQP.Q, c1OutQP.Q)
	}

	if cnt%PiOverF == 0 {
		ringP.ReduceLvl(levelP, c0OutQP.P, c0OutQP.P)
		ringP.ReduceLvl(levelP, c1OutQP.P, c1OutQP.P)
	}

	eval.Baseconverter.ModDownQPtoQNTT(levelQ, levelP, c0OutQP.Q, c0OutQP.P, c0OutQP.Q) // sum(phi(c0 * P + d0_QP))/P
	eval.Baseconverter.ModDownQPtoQNTT(levelQ, levelP, c1OutQP.Q, c1OutQP.P, c1OutQP.Q) // sum(phi(d1_QP))/P

	if state { // Rotation by zero
		ringQ.MulCoeffsMontgomeryAndAddLvl(levelQ, matrix.Vec[0].Q, ctInTmp0, c0OutQP.Q) // ctOut += c0_Q * plaintext
		ringQ.MulCoeffsMontgomeryAndAddLvl(levelQ, matrix.Vec[0].Q, ctInTmp1, c1OutQP.Q) // ctOut += c1_Q * plaintext
	}

	ctOut.Scale = matrix.Scale * ctIn.Scale
}

// MultiplyByDiagMatrixBSGS multiplies the ciphertext "ctIn" by the plaintext matrix "matrix" and returns the result on the ciphertext
// "ctOut". Memory pools for the decomposed ciphertext PoolDecompQ, PoolDecompP must be provided, those are list of poly of ringQ and ringP
// respectively, each of size params.Beta().
// The BSGS approach is used (double hoisting with baby-step giant-step), which is faster than MultiplyByDiagMatrix
// for matrix with more than a few non-zero diagonals and uses much less keys.
func (eval *evaluator) MultiplyByDiagMatrixBSGS(ctIn *Ciphertext, matrix PtDiagMatrix, PoolDecompQP []rlwe.PolyQP, ctOut *Ciphertext) {

	// N1*N2 = N
	N1 := matrix.N1

	ringQ := eval.params.RingQ()
	ringP := eval.params.RingP()
	ringQP := rlwe.RingQP{RingQ: ringQ, RingP: ringP}

	levelQ := utils.MinInt(ctOut.Level(), utils.MinInt(ctIn.Level(), matrix.Level))
	levelP := len(ringP.Modulus) - 1

	QiOverF := eval.params.QiOverflowMargin(levelQ)
	PiOverF := eval.params.PiOverflowMargin(levelP)

	// Computes the rotations indexes of the non-zero rows of the diagonalized DFT matrix for the baby-step giang-step algorithm

	index, rotations := bsgsIndex(matrix.Vec, 1<<matrix.LogSlots, matrix.N1)

	ring.CopyValuesLvl(levelQ, ctIn.Value[0], eval.ctxpool.Value[0])
	ring.CopyValuesLvl(levelQ, ctIn.Value[1], eval.ctxpool.Value[1])
	ctInTmp0, ctInTmp1 := eval.ctxpool.Value[0], eval.ctxpool.Value[1]

	// Pre-rotates ciphertext for the baby-step giant-step algorithm, does not divide by P yet
	ctInRotQP := eval.RotateHoistedNoModDownNew(levelQ, rotations, ctInTmp0, eval.PoolDecompQP)

	// Accumulator inner loop
	tmp0QP := eval.Pool[1]
	tmp1QP := eval.Pool[2]

	// Accumulator outer loop
	c0QP := eval.Pool[3]
	c1QP := eval.Pool[4]

	// Result in QP
	c0OutQP := rlwe.PolyQP{Q: ctOut.Value[0], P: eval.Pool[5].Q}
	c1OutQP := rlwe.PolyQP{Q: ctOut.Value[1], P: eval.Pool[5].P}

	ringQ.MulScalarBigintLvl(levelQ, ctInTmp0, ringP.ModulusBigint, ctInTmp0) // P*c0
	ringQ.MulScalarBigintLvl(levelQ, ctInTmp1, ringP.ModulusBigint, ctInTmp1) // P*c1

	// OUTER LOOP
	var cnt0 int
	for j := range index {
		// INNER LOOP
		var cnt1 int
		for _, i := range index[j] {
			if i == 0 {
				if cnt1 == 0 {
					ringQ.MulCoeffsMontgomeryConstantLvl(levelQ, matrix.Vec[N1*j].Q, ctInTmp0, tmp0QP.Q)
					ringQ.MulCoeffsMontgomeryConstantLvl(levelQ, matrix.Vec[N1*j].Q, ctInTmp1, tmp1QP.Q)
					tmp0QP.P.Zero()
					tmp1QP.P.Zero()
				} else {
					ringQ.MulCoeffsMontgomeryConstantAndAddNoModLvl(levelQ, matrix.Vec[N1*j].Q, ctInTmp0, tmp0QP.Q)
					ringQ.MulCoeffsMontgomeryConstantAndAddNoModLvl(levelQ, matrix.Vec[N1*j].Q, ctInTmp1, tmp1QP.Q)
				}
			} else {
				if cnt1 == 0 {
					ringQP.MulCoeffsMontgomeryConstantLvl(levelQ, levelP, matrix.Vec[N1*j+i], ctInRotQP[i][0], tmp0QP)
					ringQP.MulCoeffsMontgomeryConstantLvl(levelQ, levelP, matrix.Vec[N1*j+i], ctInRotQP[i][1], tmp1QP)
				} else {
					ringQP.MulCoeffsMontgomeryConstantAndAddNoModLvl(levelQ, levelP, matrix.Vec[N1*j+i], ctInRotQP[i][0], tmp0QP)
					ringQP.MulCoeffsMontgomeryConstantAndAddNoModLvl(levelQ, levelP, matrix.Vec[N1*j+i], ctInRotQP[i][1], tmp1QP)
				}
			}

			if cnt1%(QiOverF>>1) == (QiOverF>>1)-1 {
				ringQ.ReduceLvl(levelQ, tmp0QP.Q, tmp0QP.Q)
				ringQ.ReduceLvl(levelQ, tmp1QP.Q, tmp1QP.Q)
			}

			if cnt1%(PiOverF>>1) == (PiOverF>>1)-1 {
				ringP.ReduceLvl(levelP, tmp0QP.P, tmp0QP.P)
				ringP.ReduceLvl(levelP, tmp1QP.P, tmp1QP.P)
			}

			cnt1++
		}

		if cnt1%(QiOverF>>1) != 0 {
			ringQ.ReduceLvl(levelQ, tmp0QP.Q, tmp0QP.Q)
			ringQ.ReduceLvl(levelQ, tmp1QP.Q, tmp1QP.Q)
		}

		if cnt1%(PiOverF>>1) != 0 {
			ringP.ReduceLvl(levelP, tmp0QP.P, tmp0QP.P)
			ringP.ReduceLvl(levelP, tmp1QP.P, tmp1QP.P)
		}

		// If j != 0, then rotates ((tmp0QP.Q, tmp0QP.P), (tmp1QP.Q, tmp1QP.P)) by N1*j and adds the result on ((c0QP.Q, c0QP.P), (c1QP.Q, c1QP.P))
		if j != 0 {

			// Hoisting of the ModDown of sum(sum(phi(d1) * plaintext))
			eval.Baseconverter.ModDownQPtoQNTT(levelQ, levelP, tmp1QP.Q, tmp1QP.P, tmp1QP.Q) // c1 * plaintext + sum(phi(d1) * plaintext) + phi(c1) * plaintext mod Q

			galEl := eval.params.GaloisElementForColumnRotationBy(N1 * j)

			rtk, generated := eval.rtks.Keys[galEl]
			if !generated {
				panic("switching key not available")
			}

			index := eval.permuteNTTIndex[galEl]

			tmp1QP.Q.IsNTT = true
			eval.SwitchKeysInPlaceNoModDown(levelQ, tmp1QP.Q, rtk, c0QP.Q, c0QP.P, c1QP.Q, c1QP.P) // Switchkey(P*phi(tmpRes_1)) = (d0, d1) in base QP

			ringQP.AddLvl(levelQ, levelP, c0QP, tmp0QP, c0QP)

			// Outer loop rotations
			if cnt0 == 0 {
				ringQP.PermuteNTTWithIndexLvl(levelQ, levelP, c0QP, index, c0OutQP)
				ringQP.PermuteNTTWithIndexLvl(levelQ, levelP, c1QP, index, c1OutQP)
			} else {
				ringQP.PermuteNTTWithIndexAndAddNoModLvl(levelQ, levelP, c0QP, index, c0OutQP)
				ringQP.PermuteNTTWithIndexAndAddNoModLvl(levelQ, levelP, c1QP, index, c1OutQP)
			}

			// Else directly adds on ((c0QP.Q, c0QP.P), (c1QP.Q, c1QP.P))
		} else {
			if cnt0 == 0 {
				ringQP.CopyValuesLvl(levelQ, levelP, tmp0QP, c0OutQP)
				ringQP.CopyValuesLvl(levelQ, levelP, tmp1QP, c1OutQP)
			} else {
				ringQP.AddNoModLvl(levelQ, levelP, c0OutQP, tmp0QP, c0OutQP)
				ringQP.AddNoModLvl(levelQ, levelP, c1OutQP, tmp1QP, c1OutQP)
			}
		}

		if cnt0%QiOverF == QiOverF-1 {
			ringQ.ReduceLvl(levelQ, ctOut.Value[0], ctOut.Value[0])
			ringQ.ReduceLvl(levelQ, ctOut.Value[1], ctOut.Value[1])
		}

		if cnt0%PiOverF == PiOverF-1 {
			ringP.ReduceLvl(levelP, c0OutQP.P, c0OutQP.P)
			ringP.ReduceLvl(levelP, c1OutQP.P, c1OutQP.P)
		}

		cnt0++
	}

	if cnt0%QiOverF != 0 {
		ringQ.ReduceLvl(levelQ, ctOut.Value[0], ctOut.Value[0])
		ringQ.ReduceLvl(levelQ, ctOut.Value[1], ctOut.Value[1])
	}

	if cnt0%PiOverF != 0 {
		ringP.ReduceLvl(levelP, c0OutQP.P, c0OutQP.P)
		ringP.ReduceLvl(levelP, c1OutQP.P, c1OutQP.P)
	}

	eval.Baseconverter.ModDownQPtoQNTT(levelQ, levelP, ctOut.Value[0], c0OutQP.P, ctOut.Value[0]) // sum(phi(c0 * P + d0_QP))/P
	eval.Baseconverter.ModDownQPtoQNTT(levelQ, levelP, ctOut.Value[1], c1OutQP.P, ctOut.Value[1]) // sum(phi(d1_QP))/P

	ctOut.Scale = matrix.Scale * ctIn.Scale

	ctInRotQP = nil
	runtime.GC()
}
