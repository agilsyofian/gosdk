// Command field-encryption is a runnable, self-contained example of the
// field-level protocol in security/crypto (fields.go / encrypt_fields.go
// / jwks.go), showing exactly the two transformations requested:
//
//  1. Encrypt: plaintext JSON body -> JSON body with only the
//     configured fields replaced by encrypted values.
//  2. Decrypt: that encrypted JSON body (as received over HTTP, with
//     its accompanying headers) -> the original plaintext JSON body.
//
// Run it with:
//
//	go run ./examples/field-encryption
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"log"

	"github.com/agilsyofian/gosdk/utils/crypto"
)

func main() {
	ctx := context.Background()

	// --------------------------------------------------------------
	// 0. Setup: a KeyProvider backed by a JWKS + its private key.
	//
	// In a real service this is done once at startup: the JWKS comes
	// from config (encryption.jwks in YAML, per STEERFILE §5) and the
	// private key comes from wherever this deployment keeps secrets
	// (Vault, KMS, a PEM file) — never from the JWKS itself.
	// --------------------------------------------------------------
	const kid = "encryption-2026-08"

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("generate rsa key: %v", err)
	}

	jwks := crypto.Jwks{Keys: []crypto.Key{crypto.PublicKeyToJWK(&privKey.PublicKey, kid)}}
	keys, err := crypto.NewJWKSKeyProvider(jwks)
	if err != nil {
		log.Fatalf("load jwks: %v", err)
	}
	if err := keys.SetPrivateKey(kid, privKey); err != nil {
		log.Fatalf("attach private key: %v", err)
	}
	if err := keys.SetActive(kid); err != nil {
		log.Fatalf("set active key: %v", err)
	}

	fieldsToEncrypt := []string{"nik", "phone"}

	// ================================================================
	// Contoh 1: Encrypt
	// ================================================================
	fmt.Println("=== Contoh 1: Encrypt ===")

	plainInput := map[string]any{
		"name":    "Budi",
		"nik":     "1234567890123456",
		"phone":   "08123456789",
		"address": "Jakarta",
	}
	fmt.Println("\ninput:")
	printJSON(plainInput)

	enc := crypto.NewFieldEncryptor(keys)
	encryptedReq, err := enc.Encrypt(ctx, kid, plainInput, fieldsToEncrypt)
	if err != nil {
		log.Fatalf("encrypt: %v", err)
	}

	fmt.Println("\noutput (JSON body):")
	printJSON(encryptedReq.Body)

	fmt.Println("\noutput (headers to send alongside the body):")
	fmt.Printf("  X-Encryption-Version: %s\n", encryptedReq.Headers.Version)
	fmt.Printf("  X-Encryption-Kid:     %s\n", encryptedReq.Headers.KeyID)
	fmt.Printf("  X-Encryption-Key:     %s\n", truncate(encryptedReq.Headers.EncryptedKey, 40))
	fmt.Printf("  X-Nonce:              %s\n", encryptedReq.Headers.Nonce)
	fmt.Printf("  X-Timestamp:          %s\n", encryptedReq.Headers.Timestamp)

	// ================================================================
	// Contoh 2: Decrypt
	//
	// This is what the backend receives: encryptedReq.Body as the raw
	// HTTP request body, and encryptedReq.Headers as the request's
	// HTTP headers. (In a real handler these come from r.Body and
	// r.Header.Get, via crypto.FieldHeadersFromLookup.)
	// ================================================================
	fmt.Println("\n=== Contoh 2: Decrypt ===")

	encryptedInputBytes, err := json.Marshal(encryptedReq.Body)
	if err != nil {
		log.Fatalf("marshal encrypted body: %v", err)
	}
	fmt.Println("\ninput (JSON body as received by the backend):")
	printRawJSON(encryptedInputBytes)

	dec := crypto.NewFieldDecryptor(keys)
	decryptedBytes, err := dec.DecryptJSONFields(ctx, encryptedReq.Headers, encryptedInputBytes, fieldsToEncrypt)
	if err != nil {
		code, _ := crypto.CodeOf(err)
		log.Fatalf("decrypt failed (code=%s): %v", code, err)
	}

	fmt.Println("\noutput (restored plaintext body, ready for your existing DTO):")
	printRawJSON(decryptedBytes)
}

func printJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	fmt.Println(string(b))
}

func printRawJSON(raw []byte) {
	var v map[string]json.RawMessage
	if err := json.Unmarshal(raw, &v); err != nil {
		log.Fatalf("unmarshal: %v", err)
	}
	printJSON(v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}
