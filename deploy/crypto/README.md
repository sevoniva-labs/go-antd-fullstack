# Crypto Device Evidence Contract

The scaffold keeps KMS, HSM, password-device, SM2 certificate, and GM-TLS implementations in an `Adapter slot`. This directory provides a machine-checkable evidence format; the example remains `Not certified` and contains no device secret, PIN, private key, or token.

Run the non-certifying format checks with:

```bash
FORGE_CRYPTO_EVIDENCE_FILE=deploy/crypto/evidence.example.json make crypto-evidence-check
```

A target can only be labeled `Target-tested` after the exact product, firmware, CPU, OS, runtime, algorithms, key rotation, backup/restore, dual-control, audit, and TLS controls pass and the referenced report files are present below `FORGE_CRYPTO_EVIDENCE_ROOT` with matching lowercase SHA-256 digests:

```bash
FORGE_CRYPTO_EVIDENCE_FILE=/approved/evidence/crypto.json \
FORGE_CRYPTO_EVIDENCE_ROOT=/approved/evidence \
make crypto-evidence-check-certified
```

The checker rejects secret-bearing fields and never treats software SM3/SM4 tests as HSM/KMS or commercial-cryptography certification.
