package drlwe

import (
	"math"

	"github.com/tuneinsight/lattigo/v4/rlwe"
)

// NoiseRelinearizationKey returns the standard deviation of the noise of each individual elements in the collective RelinearizationKey.
func NoiseRelinearizationKey(params rlwe.Parameters, nbParties int) (std float64) {

	// rlk noise = [s*e0 + u*e1 + e2 + e3]
	//
	// s  = sum(s_i)
	// u  = sum(u_i)
	// e0 = sum(e_i0)
	// e1 = sum(e_i1)
	// e2 = sum(e_i2)
	// e3 = sum(e_i3)

	H := float64(nbParties * params.HammingWeight())          // var(sk) and var(u)
	e := float64(nbParties) * params.Sigma() * params.Sigma() // var(e0), var(e1), var(e2), var(e3)

	// var([s*e0 + u*e1 + e2 + e3]) = H*e + H*e + e + e = e(2H+2) = 2e(H+1)
	return math.Sqrt(2 * e * (H + 1))
}

// NoiseGaloisKey returns the standard deviation of the noise of each individual elements in a collective GaloisKey.
func NoiseGaloisKey(params rlwe.Parameters, nbParties int) (std float64) {
	return math.Sqrt(float64(nbParties)) * params.Sigma()
}

// NoiseCKS returns the standard deviation of the noise of a ciphertext after the CKS protocol
func NoiseCKS(params rlwe.Parameters, nbParties int, noisect, noiseflood float64) (std float64) {
	// #Parties * (noiseflood + noiseFreshSK) + noise ct
	return noiseDecryptWithSmudging(nbParties, noisect, params.NoiseFreshSK(), noiseflood)
}

func NoisePCKS(params rlwe.Parameters, nbParties int, noisect, noiseflood float64) (std float64) {
	// #Parties * (var(freshZeroPK) + var(noiseFlood)) + noise ct
	return noiseDecryptWithSmudging(nbParties, noisect, params.NoiseFreshPK(), noiseflood)
}

func noiseDecryptWithSmudging(nbParties int, noisect, noisefresh, noiseflood float64) (std float64) {
	std = noisefresh
	std *= std
	std += noiseflood * noiseflood
	std *= float64(nbParties)
	std += noisect * noisect
	return math.Sqrt(std)
}