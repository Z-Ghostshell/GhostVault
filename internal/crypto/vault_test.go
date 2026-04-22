package crypto

import (
	"testing"
)

func TestWrapUnwrapSealOpen(t *testing.T) {
	vc := NewVaultCrypto()
	dek, err := vc.NewDEK()
	if err != nil {
		t.Fatal(err)
	}
	w, err := vc.WrapDEK("correct horse battery staple", dek)
	if err != nil {
		t.Fatal(err)
	}
	got, err := vc.UnwrapDEK("correct horse battery staple", w)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(dek) {
		t.Fatal("dek mismatch")
	}
	nonce, ct, err := vc.SealChunk(dek, []byte("hello vault"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := vc.OpenChunk(dek, nonce, ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "hello vault" {
		t.Fatalf("got %q", plain)
	}
}
