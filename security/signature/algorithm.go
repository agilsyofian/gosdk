package signature

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// Algorithm represents a supported signature algorithm.
type Algorithm string

const (
	AlgorithmHMACSHA256 Algorithm = "HMAC-SHA256"
	AlgorithmHMACSHA512 Algorithm = "HMAC-SHA512"
	AlgorithmRSASHA256  Algorithm = "RSA-SHA256"
)

// Encoding represents the output encoding for generated signatures.
type Encoding string

const (
	EncodingHex    Encoding = "hex"
	EncodingBase64 Encoding = "base64"
)

var (
	ErrUnsupportedAlgorithm = errors.New("signature: unsupported algorithm")
	ErrUnsupportedEncoding  = errors.New("signature: unsupported encoding")
	ErrInvalidKey           = errors.New("signature: invalid key")
)

// EncodeBytes encodes binary signature into the specified encoding string.
func EncodeBytes(raw []byte, enc Encoding) (string, error) {
	switch strings.ToLower(string(enc)) {
	case string(EncodingHex):
		return hex.EncodeToString(raw), nil
	case string(EncodingBase64):
		return base64.StdEncoding.EncodeToString(raw), nil
	default:
		return "", ErrUnsupportedEncoding
	}
}

// DecodeString decodes an encoded signature string into raw bytes.
func DecodeString(sig string, enc Encoding) ([]byte, error) {
	switch strings.ToLower(string(enc)) {
	case string(EncodingHex):
		return hex.DecodeString(sig)
	case string(EncodingBase64):
		return base64.StdEncoding.DecodeString(sig)
	default:
		return nil, ErrUnsupportedEncoding
	}
}

// Sign computes the cryptographic signature of payload using key and the chosen algorithm and encoding.
func Sign(algo Algorithm, payload []byte, key []byte, enc Encoding) (string, error) {
	var rawSig []byte
	var err error

	switch algo {
	case AlgorithmHMACSHA256:
		mac := hmac.New(sha256.New, key)
		mac.Write(payload)
		rawSig = mac.Sum(nil)

	case AlgorithmHMACSHA512:
		mac := hmac.New(sha512.New, key)
		mac.Write(payload)
		rawSig = mac.Sum(nil)

	case AlgorithmRSASHA256:
		rawSig, err = signRSA(payload, key)
		if err != nil {
			return "", err
		}

	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, algo)
	}

	return EncodeBytes(rawSig, enc)
}

// Verify checks whether signature matches the payload using constant-time comparison.
func Verify(algo Algorithm, payload []byte, signature string, key []byte, enc Encoding) bool {
	sigBytes, err := DecodeString(signature, enc)
	if err != nil || len(sigBytes) == 0 {
		return false
	}

	switch algo {
	case AlgorithmHMACSHA256:
		mac := hmac.New(sha256.New, key)
		mac.Write(payload)
		expected := mac.Sum(nil)
		return subtle.ConstantTimeCompare(sigBytes, expected) == 1

	case AlgorithmHMACSHA512:
		mac := hmac.New(sha512.New, key)
		mac.Write(payload)
		expected := mac.Sum(nil)
		return subtle.ConstantTimeCompare(sigBytes, expected) == 1

	case AlgorithmRSASHA256:
		return verifyRSA(payload, sigBytes, key)

	default:
		return false
	}
}

func signRSA(payload []byte, pemKey []byte) ([]byte, error) {
	block, _ := pem.Decode(pemKey)
	var privKey *rsa.PrivateKey
	var err error

	if block != nil {
		pkcs8Key, p8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if p8Err == nil {
			if rsaKey, ok := pkcs8Key.(*rsa.PrivateKey); ok {
				privKey = rsaKey
			}
		}
		if privKey == nil {
			privKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		}
	} else {
		privKey, err = x509.ParsePKCS1PrivateKey(pemKey)
	}

	if err != nil || privKey == nil {
		return nil, ErrInvalidKey
	}

	hash := sha256.Sum256(payload)
	return rsa.SignPKCS1v15(nil, privKey, crypto.SHA256, hash[:])
}

func verifyRSA(payload []byte, sigBytes []byte, pemKey []byte) bool {
	block, _ := pem.Decode(pemKey)
	var pubKey *rsa.PublicKey

	if block != nil {
		parsedKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err == nil {
			if rsaKey, ok := parsedKey.(*rsa.PublicKey); ok {
				pubKey = rsaKey
			}
		}
		if pubKey == nil {
			pubKey, _ = x509.ParsePKCS1PublicKey(block.Bytes)
		}
	}

	if pubKey == nil {
		return false
	}

	hash := sha256.Sum256(payload)
	return rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hash[:], sigBytes) == nil
}
