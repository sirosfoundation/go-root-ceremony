package main

import "fmt"

// commands_pkcs11.go builds shell command blocks for PKCS#11 HSM operations.
// The primary target is SoftHSM 2 for testing, but the commands use standard
// pkcs11-tool (OpenSC) and should work with any compliant PKCS#11 module.

// CmdVerifyPKCS11Software returns commands to verify PKCS#11 tooling is installed.
func CmdVerifyPKCS11Software() []string {
	return []string{
		"# Verify PKCS#11 tools",
		"which softhsm2-util pkcs11-tool openssl",
		"",
		"softhsm2-util --version",
		"pkcs11-tool --version",
	}
}

// CmdVerifyPKCS11Token returns commands to initialise and verify a PKCS#11 token.
func CmdVerifyPKCS11Token(modulePath, tokenLabel string) []string {
	return []string{
		"# Set PKCS#11 module path",
		fmt.Sprintf(`PKCS11_MODULE="%s"`, modulePath),
		fmt.Sprintf(`TOKEN_LABEL="%s"`, tokenLabel),
		"",
		"# List available slots",
		`pkcs11-tool --module "${PKCS11_MODULE}" --list-slots`,
		"",
		"# Initialise token (enter SO PIN and User PIN when prompted)",
		`softhsm2-util --init-token --free --label "${TOKEN_LABEL}"`,
		"",
		"# Verify the new token is accessible",
		`pkcs11-tool --module "${PKCS11_MODULE}" --list-token-slots`,
	}
}

// CmdPKCS11GenerateKey returns commands to generate a CA signing key inside the HSM.
func CmdPKCS11GenerateKey(modulePath, tokenLabel, caName string) []string {
	return []string{
		fmt.Sprintf(`PKCS11_MODULE="%s"`, modulePath),
		fmt.Sprintf(`TOKEN_LABEL="%s"`, tokenLabel),
		"",
		"# Generate ECDSA P-384 key pair inside the HSM",
		"# Key is generated ON-DEVICE — private key never leaves the token",
		`pkcs11-tool --module "${PKCS11_MODULE}" \`,
		`  --login --token-label "${TOKEN_LABEL}" \`,
		"  --keypairgen --key-type EC:secp384r1 \\",
		fmt.Sprintf(`  --id 01 --label "%s Signing Key"`, caName),
		"# Enter User PIN when prompted",
		"",
		"# Export public key (DER format)",
		`pkcs11-tool --module "${PKCS11_MODULE}" \`,
		`  --login --token-label "${TOKEN_LABEL}" \`,
		"  --read-object --type pubkey --id 01 -o root-ca-pubkey.der",
		"",
		"# Convert to PEM",
		"openssl ec -pubin -inform DER -in root-ca-pubkey.der -outform PEM -out root-ca-pubkey.pem",
		"cat root-ca-pubkey.pem",
	}
}

// CmdPKCS11GenerateWrapKey returns commands to generate a wrap key in RAM
// for splitting, then import it into the PKCS#11 token.
func CmdPKCS11GenerateWrapKey(modulePath, tokenLabel, caName string) []string {
	return []string{
		fmt.Sprintf(`PKCS11_MODULE="%s"`, modulePath),
		fmt.Sprintf(`TOKEN_LABEL="%s"`, tokenLabel),
		"",
		"# Generate 256-bit (32-byte) random wrap key from kernel CSPRNG",
		`WRAP_KEY=$(dd if=/dev/urandom bs=32 count=1 2>/dev/null | xxd -p | tr -d '\n')`,
		"",
		"# Verify length — must be exactly 64 hex characters",
		`echo "Key length (chars): $(echo -n "${WRAP_KEY}" | wc -c)"`,
		"# Expected: 64",
		"",
		"# Show only first 4 and last 4 hex chars for witness confirmation",
		`echo "WRAP KEY fingerprint: ${WRAP_KEY:0:4}....${WRAP_KEY: -4}"`,
		"",
		"# The key will be split via SSS in the next section and",
		"# imported into the PKCS#11 token after verification.",
	}
}

// CmdPKCS11ImportWrapKey returns commands to import the wrap key into the token.
func CmdPKCS11ImportWrapKey(modulePath, tokenLabel, caName string) []string {
	return []string{
		fmt.Sprintf(`PKCS11_MODULE="%s"`, modulePath),
		fmt.Sprintf(`TOKEN_LABEL="%s"`, tokenLabel),
		"",
		"# Write wrap key to binary file (temporary — will be shredded)",
		`echo -n "${WRAP_KEY}" | xxd -r -p > wrap-key.bin`,
		"",
		fmt.Sprintf("# Import wrap key as AES-256 secret key into %s", tokenLabel),
		`pkcs11-tool --module "${PKCS11_MODULE}" \`,
		`  --login --token-label "${TOKEN_LABEL}" \`,
		"  --write-object wrap-key.bin --type secrkey \\",
		fmt.Sprintf(`  --id 0100 --label "%s Wrap Key"`, caName),
		"# Enter User PIN when prompted",
		"",
		"# Verify the wrap key object was imported",
		`pkcs11-tool --module "${PKCS11_MODULE}" \`,
		`  --login --token-label "${TOKEN_LABEL}" \`,
		"  --list-objects --type secrkey",
		"",
		"# Securely delete the temporary binary file",
		"shred -u wrap-key.bin",
	}
}
