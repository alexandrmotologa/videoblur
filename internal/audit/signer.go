package audit

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

// KeyPair holds public and private Ed25519 cryptographic keys.
type KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// GenerateKeyPair generates a new Ed25519 keypair for forensic compliance signing.
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating ed25519 key: %w", err)
	}
	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// PublicKeyToPEM serializes an Ed25519 public key to PKIX PEM format.
func (kp *KeyPair) PublicKeyToPEM() (string, error) {
	bytes, err := x509.MarshalPKIXPublicKey(kp.PublicKey)
	if err != nil {
		return "", err
	}
	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: bytes,
	}
	return string(pem.EncodeToMemory(block)), nil
}

// SignMessage signs payload bytes using the private key, returning hex signature.
func (kp *KeyPair) SignMessage(data []byte) string {
	digest := sha256.Sum256(data)
	sig := ed25519.Sign(kp.PrivateKey, digest[:])
	return hex.EncodeToString(sig)
}

// VerifySignature validates a hex signature against the payload using a public key.
func VerifySignature(pubKey ed25519.PublicKey, data []byte, sigHex string) (bool, error) {
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false, fmt.Errorf("decoding hex signature: %w", err)
	}

	digest := sha256.Sum256(data)
	valid := ed25519.Verify(pubKey, digest[:], sigBytes)
	return valid, nil
}

// ParsePublicKeyPEM parses an Ed25519 public key from PEM string.
func ParsePublicKeyPEM(pemStr string) (ed25519.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %w", err)
	}

	edPub, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("not an Ed25519 public key")
	}

	return edPub, nil
}

// SaveKeysToFile writes private and public keys to disk.
func (kp *KeyPair) SaveKeysToFile(privPath, pubPath string) error {
	privBytes, err := x509.MarshalPKCS8PrivateKey(kp.PrivateKey)
	if err != nil {
		return err
	}
	privBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privBlock), 0600); err != nil {
		return err
	}

	pubPEM, err := kp.PublicKeyToPEM()
	if err != nil {
		return err
	}
	return os.WriteFile(pubPath, []byte(pubPEM), 0644)
}

// LoadKeyPair loads private and public keys from disk.
func LoadKeyPair(privPath, pubPath string) (*KeyPair, error) {
	privPEM, err := os.ReadFile(privPath)
	if err != nil {
		return nil, err
	}
	privBlock, _ := pem.Decode(privPEM)
	if privBlock == nil {
		return nil, errors.New("invalid private key PEM")
	}
	privParsed, err := x509.ParsePKCS8PrivateKey(privBlock.Bytes)
	if err != nil {
		return nil, err
	}
	privKey, ok := privParsed.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("not an Ed25519 private key")
	}

	pubPEM, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, err
	}
	pubKey, err := ParsePublicKeyPEM(string(pubPEM))
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		PrivateKey: privKey,
		PublicKey:  pubKey,
	}, nil
}
