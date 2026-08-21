package crypto

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/emmansun/gmsm/sm2"
)

// Signer is the application boundary for message signatures. A KMS, HSM, or
// password-device adapter may implement the same contract without exposing
// private key material to the application process.
type Signer interface {
	Algorithm() string
	Sign(ctx context.Context, message []byte) ([]byte, error)
	Verify(ctx context.Context, message, signature []byte) error
}

// SM2SoftwareSigner is a development and test baseline only. Production code
// must inject an approved device-backed Signer through the adapter slot.
type SM2SoftwareSigner struct {
	privateKey *sm2.PrivateKey
	uid        []byte
}

func NewSM2SoftwareSigner(rawPrivateKey, uid []byte) (*SM2SoftwareSigner, error) {
	if len(rawPrivateKey) != 32 {
		return nil, errors.New("SM2 software signer requires a 32-byte private key")
	}
	if len(uid) == 0 {
		return nil, errors.New("SM2 software signer requires an explicit user ID")
	}
	privateKey, err := sm2.NewPrivateKey(rawPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("parse SM2 private key: %w", err)
	}
	return &SM2SoftwareSigner{privateKey: privateKey, uid: append([]byte(nil), uid...)}, nil
}

func (s *SM2SoftwareSigner) Algorithm() string { return "SM2-SM3" }

func (s *SM2SoftwareSigner) Sign(ctx context.Context, message []byte) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if len(message) == 0 {
		return nil, errors.New("message is required")
	}
	signature, err := s.privateKey.SignWithSM2(rand.Reader, s.uid, message)
	if err != nil {
		return nil, fmt.Errorf("SM2 sign: %w", err)
	}
	return signature, nil
}

func (s *SM2SoftwareSigner) Verify(ctx context.Context, message, signature []byte) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if len(message) == 0 || len(signature) == 0 {
		return errors.New("message and signature are required")
	}
	if !sm2.VerifyASN1WithSM2(&s.privateKey.PublicKey, s.uid, message, signature) {
		return errors.New("SM2 signature verification failed")
	}
	return nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}
