package security

import (
	"fmt"

	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	crypto "github.com/agilsyofian/gosdk/utils/crypto"
)

// FieldDecryptor initializes a crypto.FieldDecryptor using the application
// encryption key configuration (kid, private key, and JWKS).
func FieldDecryptor(cfg config.AppConfig) (*crypto.FieldDecryptor, error) {
	encKid := cfg.Security.Kid.Encryption

	var encKeys []crypto.Key
	for _, k := range cfg.Security.JWKS.Keys {
		if k.Use == "enc" || k.Kid == encKid {
			encKeys = append(encKeys, crypto.Key{
				Use: k.Use,
				Kty: k.Kty,
				Alg: k.Alg,
				Kid: k.Kid,
				N:   k.N,
				E:   k.E,
				X5c: k.X5c,
			})
		}
	}

	kp, err := crypto.NewJWKSKeyProvider(crypto.Jwks{Keys: encKeys})
	if err != nil {
		return nil, fmt.Errorf("crypto: init jwks key provider: %w", err)
	}

	if privB64, ok := cfg.Security.PrivateKey[encKid]; ok && privB64 != "" {
		privKey, err := jwt.B64ToPrivateKey(privB64)
		if err != nil {
			return nil, fmt.Errorf("crypto: parse encryption private key for kid %q: %w", encKid, err)
		}
		if err := kp.SetPrivateKey(encKid, privKey); err != nil {
			return nil, fmt.Errorf("crypto: set encryption private key: %w", err)
		}
		if err := kp.SetActive(encKid); err != nil {
			return nil, fmt.Errorf("crypto: set active encryption key: %w", err)
		}
	}

	return crypto.NewFieldDecryptor(kp), nil
}
