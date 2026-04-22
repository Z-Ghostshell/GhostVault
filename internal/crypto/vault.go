package crypto

import (
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	dekSize             = 32
	xNonceSize          = chacha20poly1305.NonceSizeX
	argonTime           = 3
	argonMemoryKB       = 65536
	argonThreads  uint8 = 4
)

var ErrInvalidCiphertext = errors.New("invalid ciphertext")

type VaultCrypto struct{}

func NewVaultCrypto() *VaultCrypto { return &VaultCrypto{} }

type WrappedDEK struct {
	Salt        []byte
	TimeCost    uint32
	MemoryKB    uint32
	Parallelism uint8
	Blob        []byte
}

func (VaultCrypto) NewDEK() ([]byte, error) {
	b := make([]byte, dekSize)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, err
	}
	return b, nil
}

func (VaultCrypto) WrapDEK(password string, dek []byte) (*WrappedDEK, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	kek := argon2.IDKey([]byte(password), salt, argonTime, argonMemoryKB, argonThreads, dekSize)
	aead, err := chacha20poly1305.NewX(kek)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, xNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := aead.Seal(nil, nonce, dek, nil)
	out := append(append([]byte{}, nonce...), sealed...)
	return &WrappedDEK{
		Salt:        salt,
		TimeCost:    argonTime,
		MemoryKB:    argonMemoryKB,
		Parallelism: argonThreads,
		Blob:        out,
	}, nil
}

func (VaultCrypto) UnwrapDEK(password string, w *WrappedDEK) ([]byte, error) {
	kek := argon2.IDKey([]byte(password), w.Salt, w.TimeCost, w.MemoryKB, w.Parallelism, dekSize)
	aead, err := chacha20poly1305.NewX(kek)
	if err != nil {
		return nil, err
	}
	if len(w.Blob) < xNonceSize {
		return nil, ErrInvalidCiphertext
	}
	nonce := w.Blob[:xNonceSize]
	sealed := w.Blob[xNonceSize:]
	plain, err := aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}
	return plain, nil
}

func (VaultCrypto) SealChunk(dek, plaintext []byte) (nonce, ciphertext []byte, err error) {
	aead, err := chacha20poly1305.NewX(dek)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, xNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	return nonce, aead.Seal(nil, nonce, plaintext, nil), nil
}

func (VaultCrypto) OpenChunk(dek, nonce, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(dek)
	if err != nil {
		return nil, err
	}
	return aead.Open(nil, nonce, ciphertext, nil)
}
