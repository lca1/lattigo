package ckks

// Decryptor is an interface for decrypting Ciphertexts. A Decryptor stores the secret-key.
type Decryptor interface {
	// DecryptNew decrypts the ciphertext and returns a newly created
	// plaintext. A Horner method is used for evaluating the decryption.
	DecryptNew(ciphertext *Ciphertext) (plaintext *Plaintext)

	// Decrypt decrypts the ciphertext and returns the result on the provided
	// receiver plaintext. A Horner method is used for evaluating the
	// decryption.
	Decrypt(ciphertext *Ciphertext, plaintext *Plaintext)
}

// decryptor is a structure used to decrypt ciphertext. It stores the secret-key.
type decryptor struct {
	params      *Parameters
	ckksContext *Context
	sk          *SecretKey
}

// NewDecryptor instantiates a new Decryptor that will be able to decrypt ciphertexts
// encrypted under the provided secret-key.
func NewDecryptor(params *Parameters, sk *SecretKey) Decryptor {
	if !params.isValid {
		panic("cannot newDecryptor: parameters are invalid (check if the generation was done properly)")
	}

	if sk.sk.GetDegree() != int(1<<params.LogN) {
		panic("cannot newDecryptor: secret_key degree must match context degree")
	}

	return &decryptor{
		params:      params.Copy(),
		ckksContext: newContext(params),
		sk:          sk,
	}
}

// DecryptNew decrypts the Ciphertext and returns a newly created Plaintext.
// Horner method is used for evaluating the decryption.
func (decryptor *decryptor) DecryptNew(ciphertext *Ciphertext) (plaintext *Plaintext) {

	plaintext = NewPlaintext(decryptor.params, ciphertext.Level(), ciphertext.Scale())

	decryptor.Decrypt(ciphertext, plaintext)

	return plaintext
}

// Decrypt decrypts the Ciphertext and returns the result on the provided receiver Plaintext.
// Horner method is used for evaluating the decryption.
func (decryptor *decryptor) Decrypt(ciphertext *Ciphertext, plaintext *Plaintext) {

	context := decryptor.ckksContext.contextQ

	level := ciphertext.Level()

	plaintext.SetScale(ciphertext.Scale())

	context.CopyLvl(level, ciphertext.value[ciphertext.Degree()], plaintext.value)

	plaintext.value.Coeffs = plaintext.value.Coeffs[:ciphertext.Level()+1]

	for i := uint64(ciphertext.Degree()); i > 0; i-- {

		context.MulCoeffsMontgomeryLvl(level, plaintext.value, decryptor.sk.sk, plaintext.value)
		context.AddLvl(level, plaintext.value, ciphertext.value[i-1], plaintext.value)

		if i&7 == 7 {
			context.ReduceLvl(level, plaintext.value, plaintext.value)
		}
	}

	if (ciphertext.Degree())&7 != 7 {
		context.ReduceLvl(level, plaintext.value, plaintext.value)
	}
}
