package sandbox

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The key files, in SANDBOX_KEYS_DIR. telebirr's calls are signed with RSA
// keys: Kiflet signs its requests with the merchant's private key and checks
// notifications with telebirr's public key, and the sandbox holds the other
// half of each pair.
const (
	MerchantPrivateKey = "merchant_private.pem"
	MerchantPublicKey  = "merchant_public.pem"
	TelebirrPrivateKey = "telebirr_private.pem"
	TelebirrPublicKey  = "telebirr_public.pem"
)

// WriteKeys makes a merchant and a telebirr key pair in dir, keeping a pair
// already there, and returns the files it wrote. They are readable by all,
// so that a sandbox in a container can read them: they are throwaway keys,
// which only the sandbox and a Kiflet pointed at it trust.
func WriteKeys(dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	for _, pair := range [][2]string{{MerchantPrivateKey, MerchantPublicKey}, {TelebirrPrivateKey, TelebirrPublicKey}} {
		private, public := filepath.Join(dir, pair[0]), filepath.Join(dir, pair[1])
		havePrivate, havePublic := exists(private), exists(public)
		if havePrivate && havePublic {
			continue
		}
		if havePrivate || havePublic {
			return written, fmt.Errorf("only half of a key pair is in %s: remove %s and %s to make a new pair", dir, pair[0], pair[1])
		}
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return written, err
		}
		privateDER, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return written, err
		}
		publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		if err != nil {
			return written, err
		}
		for _, f := range []struct {
			path, kind string
			der        []byte
		}{{private, "PRIVATE KEY", privateDER}, {public, "PUBLIC KEY", publicDER}} {
			if err := os.WriteFile(f.path, pem.EncodeToMemory(&pem.Block{Type: f.kind, Bytes: f.der}), 0o644); err != nil {
				return written, err
			}
			written = append(written, f.path)
		}
	}
	return written, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// LoadKeys reads the sandbox's halves of the key pairs from dir: the
// merchant's public key, to check Kiflet's signatures, and telebirr's
// private key, to sign as telebirr.
func LoadKeys(dir string) (*rsa.PublicKey, *rsa.PrivateKey, error) {
	hint := fmt.Sprintf(" (make them with `kiflet-sandbox keys` or `make sandbox-keys`, in %s)", dir)
	b, err := os.ReadFile(filepath.Join(dir, MerchantPublicKey))
	if err != nil {
		return nil, nil, fmt.Errorf("the merchant's public key: %w%s", err, hint)
	}
	merchant, err := parsePublic(b)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", MerchantPublicKey, err)
	}
	if b, err = os.ReadFile(filepath.Join(dir, TelebirrPrivateKey)); err != nil {
		return nil, nil, fmt.Errorf("telebirr's private key: %w%s", err, hint)
	}
	telebirr, err := parsePrivate(b)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", TelebirrPrivateKey, err)
	}
	return merchant, telebirr, nil
}

func parsePublic(b []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("not a PEM file")
	}
	if k, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PublicKey); ok {
			return rk, nil
		}
		return nil, errors.New("not an RSA key")
	}
	return x509.ParsePKCS1PublicKey(block.Bytes)
}

func parsePrivate(b []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("not a PEM file")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("not an RSA key")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// ---------------------------------------------------------------------------
// telebirr's signatures
// ---------------------------------------------------------------------------

const signType = "SHA256WithRSA"

// signedText is what a telebirr signature covers: every field but sign and
// sign_type, as name=value, sorted by name and joined with &.
func signedText(fields map[string]string) string {
	names := make([]string, 0, len(fields))
	for k := range fields {
		if k != "sign" && k != "sign_type" {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	pairs := make([]string, len(names))
	for i, k := range names {
		pairs[i] = k + "=" + fields[k]
	}
	return strings.Join(pairs, "&")
}

// sign signs fields as telebirr's guide has it: RSA-PSS with SHA-256, in
// base64. A key that can't sign gives an empty signature, which fails to
// verify where it is checked.
func sign(key *rsa.PrivateKey, fields map[string]string) string {
	digest := sha256.Sum256([]byte(signedText(fields)))
	sig, err := rsa.SignPSS(rand.Reader, key, crypto.SHA256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash})
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(sig)
}

// verify checks a signature made as sign makes them, with the other half
// of the key. It takes any salt length, which the guide doesn't fix, but
// only RSA-PSS.
func verify(public *rsa.PublicKey, fields map[string]string, signature string) bool {
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil || len(sig) == 0 {
		return false
	}
	digest := sha256.Sum256([]byte(signedText(fields)))
	return rsa.VerifyPSS(public, crypto.SHA256, digest[:], sig, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto}) == nil
}
