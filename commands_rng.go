package main

import "fmt"

// commands_rng.go builds shell command blocks for external RNG verification
// and key generation outside an HSM using a dedicated hardware RNG device.

// CmdVerifyRNGDevice returns commands to identify and verify the RNG device.
func CmdVerifyRNGDevice(rngDevice string) []string {
	return []string{
		"# ── External RNG Device Verification ──",
		fmt.Sprintf("# RNG device: %s", rngDevice),
		"",
		"# Verify the device exists and is readable",
		fmt.Sprintf(`test -r %s && echo "RNG device %s: accessible ✓" || echo "FAIL: %s not found or not readable"`, rngDevice, rngDevice, rngDevice),
		"",
		"# Identify the hardware RNG",
		`cat /sys/class/misc/hw_random/rng_current 2>/dev/null || echo "(kernel hw_random not applicable)"`,
		`cat /sys/class/misc/hw_random/rng_available 2>/dev/null || echo "(no kernel RNG info)"`,
		"",
	}
}

// CmdTestRNGEntropy returns commands to run entropy quality tests on the RNG device.
// Tests a sample of data for randomness quality before using it for key material.
func CmdTestRNGEntropy(rngDevice string) []string {
	return []string{
		"# ── RNG Entropy Quality Tests ──",
		"# Collect a test sample from the RNG device",
		fmt.Sprintf(`dd if=%s of=/tmp/ceremony/rng-sample.bin bs=1024 count=256 2>/dev/null`, rngDevice),
		`echo "Collected 256 KiB sample for entropy testing"`,
		"",
		"# Test 1: ent — basic entropy estimation",
		"# Ideal: entropy ≈ 7.999 bits/byte, chi-square 1-99%",
		`ent /tmp/ceremony/rng-sample.bin`,
		"",
		"# Test 2: rngtest (from rng-tools) — FIPS 140-2 statistical tests",
		"# Runs Monobit, Poker, Runs, and Long Runs tests",
		"# Acceptable: 0 failures in 1000+ blocks",
		fmt.Sprintf(`dd if=%s bs=2500 count=101 2>/dev/null | rngtest --blockcount=100`, rngDevice),
		"",
		"# Test 3: Compression ratio — truly random data is incompressible",
		`ORIG=$(wc -c < /tmp/ceremony/rng-sample.bin)`,
		`gzip -c /tmp/ceremony/rng-sample.bin | wc -c | awk -v orig="$ORIG" '{printf "Compression ratio: %.4f (ideal ≈ 1.00)\\n", $1/orig}'`,
		"",
		"# Test 4: Check for stuck bits (compare two independent samples)",
		fmt.Sprintf(`dd if=%s of=/tmp/ceremony/rng-sample2.bin bs=1024 count=256 2>/dev/null`, rngDevice),
		`if cmp -s /tmp/ceremony/rng-sample.bin /tmp/ceremony/rng-sample2.bin; then`,
		`  echo "FAIL: Two samples are identical — RNG may be stuck!"`,
		`  exit 1`,
		`else`,
		`  echo "CONFIRMED: Samples differ ✓"`,
		`fi`,
		"",
		"# Clean up test samples",
		"shred -u /tmp/ceremony/rng-sample.bin /tmp/ceremony/rng-sample2.bin",
	}
}

// CmdGenerateKeyFromRNG returns commands to generate a 256-bit key from the
// specified RNG device instead of /dev/urandom.
func CmdGenerateKeyFromRNG(rngDevice string) []string {
	return []string{
		"# Generate 256-bit (32-byte) wrap key from hardware RNG",
		fmt.Sprintf("# Source: %s (tested above)", rngDevice),
		fmt.Sprintf(`WRAP_KEY=$(dd if=%s bs=32 count=1 2>/dev/null | xxd -p | tr -d '\n')`, rngDevice),
		"",
		"# Verify length — must be exactly 64 hex characters",
		`echo "Key length (chars): $(echo -n "${WRAP_KEY}" | wc -c)"`,
		"# Expected: 64",
		"",
		"# Show only first 4 and last 4 hex chars for witness confirmation",
		"# The full key is NEVER displayed on screen (camera is recording)",
		`echo "WRAP KEY fingerprint: ${WRAP_KEY:0:4}....${WRAP_KEY: -4}"`,
		"",
		fmt.Sprintf("# Source confirmed: hardware RNG %s", rngDevice),
	}
}

// CmdGenerateCAKeyFromRNG returns commands to generate a CA private key externally
// using the specified RNG device, for subsequent import into an HSM.
func CmdGenerateCAKeyFromRNG(rngDevice, caName string) []string {
	return []string{
		fmt.Sprintf("# Generate EC P-384 private key using hardware RNG: %s", rngDevice),
		"# Key is generated OUTSIDE the HSM and will be imported after wrapping.",
		fmt.Sprintf(`openssl ecparam -genkey -name secp384r1 -rand %s -noout -out ca-key.pem`, rngDevice),
		"",
		"# Extract public key",
		"openssl ec -in ca-key.pem -pubout -out root-ca-pubkey.pem",
		"cat root-ca-pubkey.pem",
		"",
		"# Verify key is valid",
		"openssl ec -in ca-key.pem -check -noout",
		fmt.Sprintf(`echo "%s signing key generated from external RNG ✓"`, caName),
		"",
		"# Convert private key to DER for HSM import",
		"openssl ec -in ca-key.pem -outform DER -out ca-key.der",
		"",
		"# The private key will be imported into the HSM and then securely erased from RAM.",
	}
}

// CmdImportExternalKeyToYubiHSM returns commands to import an externally generated
// key into a YubiHSM 2.
func CmdImportExternalKeyToYubiHSM(caName string) []string {
	return []string{
		"# Import externally generated key into YubiHSM 2 FIPS",
		"yubihsm-shell \\",
		"  --action put-asymmetric-key \\",
		`  --authkey 1 --password "password" \`,
		"  --object-id 0x0001 \\",
		fmt.Sprintf(`  --label "%s Signing Key" \`, caName),
		`  --capabilities "sign-ecdsa,exportable-under-wrap" \`,
		"  --algorithm ec-p384 \\",
		"  --in ca-key.pem",
		"",
		"# Verify the key was imported",
		"yubihsm-shell \\",
		"  --action get-object-info \\",
		`  --authkey 1 --password "password" \`,
		"  --object-id 0x0001 --object-type asymmetric-key",
		"",
		"# Securely erase the plaintext private key",
		"shred -u ca-key.pem ca-key.der",
		`echo "Private key securely erased from workstation ✓"`,
	}
}

// CmdImportExternalKeyToPKCS11 returns commands to import an externally generated
// key into a PKCS#11 token.
func CmdImportExternalKeyToPKCS11(modulePath, tokenLabel, caName string) []string {
	return []string{
		fmt.Sprintf(`PKCS11_MODULE="%s"`, modulePath),
		fmt.Sprintf(`TOKEN_LABEL="%s"`, tokenLabel),
		"",
		"# Import externally generated key into PKCS#11 token",
		`pkcs11-tool --module "${PKCS11_MODULE}" \`,
		`  --login --token-label "${TOKEN_LABEL}" \`,
		"  --write-object ca-key.der --type privkey \\",
		fmt.Sprintf(`  --id 01 --label "%s Signing Key"`, caName),
		"# Enter User PIN when prompted",
		"",
		"# Verify the key was imported",
		`pkcs11-tool --module "${PKCS11_MODULE}" \`,
		`  --login --token-label "${TOKEN_LABEL}" \`,
		"  --list-objects --type privkey",
		"",
		"# Securely erase the plaintext private key",
		"shred -u ca-key.pem ca-key.der",
		`echo "Private key securely erased from workstation ✓"`,
	}
}
