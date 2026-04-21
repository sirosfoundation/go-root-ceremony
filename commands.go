package main

import "fmt"

// commands.go builds the shell command blocks that appear in the ceremony script.
// Each function returns a []string of lines suitable for rendering in a <pre> block.
// Lines beginning with '#' are treated as comments and styled accordingly.

// CmdVerifyAirGap returns commands to confirm the workstation has no network.
func CmdVerifyAirGap() []string {
	return []string{
		"# Verify no network interfaces are up",
		"ip link show",
		"# All interfaces other than loopback (lo) must show DOWN or be absent",
		"",
		`ping -c 1 8.8.8.8 || echo "CONFIRMED: No network (expected)"`,
	}
}

// CmdPrepareWorkdir returns commands to create a RAM-backed working directory.
func CmdPrepareWorkdir() []string {
	return []string{
		"# Create a RAM-based working directory (tmpfs — never written to disk)",
		"mkdir -p /tmp/ceremony",
		"chmod 700 /tmp/ceremony",
		"cd /tmp/ceremony",
		"",
		"# Verify we are on tmpfs",
		"df -T /tmp/ceremony",
		"# Type column must show 'tmpfs'",
	}
}

// CmdVerifySoftware returns commands to confirm all required tools are installed.
func CmdVerifySoftware(hsmType HSMType) []string {
	lines := []string{
		"# Verify installed tools",
		"which ssss-split ssss-combine age age-plugin-yubikey ykman",
		"",
		"# Print versions for ceremony log",
		`ssss-split --version 2>&1 | head -1`,
		"age --version",
		"age-plugin-yubikey --version",
		"ykman --version",
	}
	switch hsmType {
	case HSMYubiHSM:
		lines = append(lines, "yubihsm-shell --version")
	case HSMPKCS11:
		lines = append(lines, "softhsm2-util --version", "pkcs11-tool --version")
	}
	return lines
}

// CmdVerifyYubiHSM returns commands to detect and list YubiHSM 2 devices.
func CmdVerifyYubiHSM() []string {
	return []string{
		"# Start yubihsm-connector",
		"yubihsm-connector &",
		"",
		"# List objects on connected YubiHSM 2 devices",
		`yubihsm-shell --action list-objects --authkey 1 --password "password" 2>&1 | head -20`,
		"",
		"# Get device info and serial numbers",
		"yubihsm-shell --action get-device-info",
	}
}

// CmdEnrollYubiKey returns the enrollment commands for custodian i (0-indexed).
func CmdEnrollYubiKey(i int) []string {
	return []string{
		"# Verify YubiKey detected",
		"ykman info",
		"",
		"# Note serial number (record in ceremony log)",
		`ykman info | grep "Serial number"`,
		"",
		"# Change PIV PIN (default 123456) and PUK (default 12345678)",
		"ykman piv access change-pin",
		"ykman piv access change-puk",
		"",
		"# Generate EC P-256 key on slot 9d (Key Management) — key stays on device",
		"age-plugin-yubikey --generate --slot 4",
		"# Slot mapping: 1=9a  2=9c  3=9e  4=9d (Key Management)",
		"",
		"# List all age-compatible identities on this YubiKey",
		"age-plugin-yubikey --list",
		"",
		fmt.Sprintf(`# RECORD recipient string for Custodian %d:`, i+1),
		fmt.Sprintf(`C%d_RECIPIENT="age1yubikey1[paste full recipient string here]"`, i+1),
	}
}

// CmdGenerateWrapKey returns commands to generate a 256-bit random wrap key in RAM.
func CmdGenerateWrapKey() []string {
	return []string{
		"# Generate 256-bit (32-byte) random wrap key from kernel CSPRNG",
		`WRAP_KEY=$(dd if=/dev/urandom bs=32 count=1 2>/dev/null | xxd -p | tr -d '\n')`,
		"",
		"# Verify length — must be exactly 64 hex characters",
		`echo "Key length (chars): $(echo -n "${WRAP_KEY}" | wc -c)"`,
		"# Expected: 64",
		"",
		"# Show only first 4 and last 4 hex chars for witness confirmation",
		"# The full key is NEVER displayed on screen (camera is recording)",
		`echo "WRAP KEY fingerprint: ${WRAP_KEY:0:4}....${WRAP_KEY: -4}"`,
	}
}

// CmdGenerateYubiHSMKey returns commands to generate the CA key inside the YubiHSM.
func CmdGenerateYubiHSMKey(caName string) []string {
	return []string{
		"# Generate ECDSA P-384 signing key inside YubiHSM 2 FIPS",
		"# Key is generated ON-DEVICE — never exists in plaintext outside the HSM",
		"yubihsm-shell \\",
		"  --action generate-asymmetric-key \\",
		`  --authkey 1 --password "password" \`,
		"  --object-id 0x0001 \\",
		fmt.Sprintf(`  --label "%s Signing Key" \`, caName),
		`  --capabilities "sign-ecdsa,exportable-under-wrap" \`,
		"  --algorithm ec-p384",
		"",
		"# Export public key for CA certificate generation",
		"yubihsm-shell \\",
		"  --action get-public-key \\",
		`  --authkey 1 --password "password" \`,
		"  --object-id 0x0001 --object-type asymmetric-key \\",
		"  --out root-ca-pubkey.pem",
		"",
		"cat root-ca-pubkey.pem",
	}
}

// CmdGenerateYubiHSMWrapKey returns commands to generate a wrap key in the YubiHSM
// and also export the raw value to RAM for SSS splitting.
func CmdGenerateYubiHSMWrapKey(caName string) []string {
	return []string{
		"# Generate AES-256 wrap key inside YubiHSM 2 FIPS",
		"yubihsm-shell \\",
		"  --action generate-wrap-key \\",
		`  --authkey 1 --password "password" \`,
		"  --object-id 0x0100 \\",
		fmt.Sprintf(`  --label "%s Wrap Key" \`, caName),
		`  --capabilities "export-wrapped,import-wrapped" \`,
		`  --delegated "sign-ecdsa,exportable-under-wrap" \`,
		"  --algorithm aes256-ccm-wrap",
		"",
		"# To split the wrap key via SSS, we need its value in RAM.",
		"# Generate an equivalent key externally for the ceremony:",
		`WRAP_KEY=$(dd if=/dev/urandom bs=32 count=1 2>/dev/null | xxd -p | tr -d '\n')`,
		`echo "WRAP KEY fingerprint: ${WRAP_KEY:0:4}....${WRAP_KEY: -4}"`,
		"# This key will be imported into the YubiHSM in Section 9.",
	}
}

// CmdSSSplit returns the ssss-split command for the given N/M parameters.
func CmdSSSplit(n, m int) []string {
	return []string{
		fmt.Sprintf("# Split wrap key into %d shares, threshold %d", n, m),
		fmt.Sprintf("# -t %d  = minimum shares required to reconstruct", m),
		fmt.Sprintf("# -n %d  = total shares generated", n),
		"# -x   = hex input/output mode (required for binary secrets)",
		"# -s 256 = prime field size in bits (must be >= secret bit length)",
		"# Output is written directly to individual share files (never shown on screen)",
		fmt.Sprintf(`echo -n "${WRAP_KEY}" | ssss-split -t %d -n %d -x -s 256 | while IFS= read -r line; do`, m, n),
		`  NUM=$(echo "$line" | cut -d- -f1)`,
		`  echo "$line" > "share-${NUM}.txt"`,
		`  echo "Share ${NUM} written to share-${NUM}.txt"`,
		"done",
		"",
		fmt.Sprintf("# Confirm all %d share files exist", n),
		fmt.Sprintf(`ls -1 share-*.txt | wc -l`),
		fmt.Sprintf("# Expected: %d", n),
	}
}

// CmdEncryptShare returns the age encryption command for custodian i (0-indexed).
func CmdEncryptShare(i int, storageMethod StorageMethod) []string {
	lines := []string{
		fmt.Sprintf("# Set recipient for Custodian %d", i+1),
		fmt.Sprintf(`C%d_RECIPIENT="age1yubikey1[paste Custodian %d recipient string]"`, i+1, i+1),
		"",
		fmt.Sprintf("# Encrypt share — only Custodian %d's YubiKey can decrypt this", i+1),
		fmt.Sprintf(
			`cat "share-%d.txt" | age -r "${C%d_RECIPIENT}" -o custodian%d.share.age`,
			i+1, i+1, i+1),
		"",
		fmt.Sprintf("ls -lh custodian%d.share.age", i+1),
	}
	if storageMethod == StoragePrint || storageMethod == StorageBoth {
		lines = append(lines,
			"",
			"# Encode to base64 and generate printable QR code",
			fmt.Sprintf("base64 custodian%d.share.age | qrencode -o custodian%d-qr.png -t PNG", i+1, i+1),
			fmt.Sprintf("lp custodian%d-qr.png  # or open in image viewer to print", i+1),
		)
	}
	return lines
}

// CmdVerifyReconstruct returns commands to test SSS reconstruction with M shares.
func CmdVerifyReconstruct(m int) []string {
	lines := []string{
		"# Decrypt test shares — each participating custodian inserts their YubiKey",
		"# age-plugin-yubikey will prompt for PIV PIN",
	}
	for i := 0; i < m; i++ {
		lines = append(lines,
			"",
			fmt.Sprintf("# Custodian %d — insert YubiKey, then:", i+1),
			fmt.Sprintf(
				"age-plugin-yubikey --identity | age --decrypt -i - custodian%d.share.age > share%d.txt",
				i+1, i+1),
		)
	}
	shareFiles := ""
	for i := 0; i < m; i++ {
		if i > 0 {
			shareFiles += " "
		}
		shareFiles += fmt.Sprintf("share%d.txt", i+1)
	}
	lines = append(lines,
		"",
		fmt.Sprintf("# Combine the %d decrypted shares", m),
		fmt.Sprintf("cat %s | ssss-combine -t %d -x > reconstructed.txt", shareFiles, m),
		"",
		"# Automated comparison — secrets are NOT displayed on screen",
		`if [ "$(cat reconstructed.txt)" = "${WRAP_KEY}" ]; then`,
		`  echo "VERIFICATION PASSED — reconstructed key matches original"`,
		"else",
		`  echo "VERIFICATION FAILED — keys do not match"`,
		"fi",
		"",
		"# Wipe reconstruction artifact",
		"shred -u reconstructed.txt",
	)
	return lines
}

// CmdRecoveryDecrypt returns commands for a recovery ceremony.
func CmdRecoveryDecrypt(i int) []string {
	return []string{
		fmt.Sprintf("# Custodian %d — insert YubiKey and enter PIV PIN when prompted", i+1),
		fmt.Sprintf(
			"age-plugin-yubikey --identity | age --decrypt -i - custodian%d.share.age",
			i+1),
		"# Note the decrypted share value and give it to the Operator",
	}
}

// CmdRecoveryCombine returns the ssss-combine command for a recovery ceremony.
func CmdRecoveryCombine(m int) []string {
	return []string{
		fmt.Sprintf("# Combine %d shares to reconstruct the secret", m),
		fmt.Sprintf("ssss-combine -t %d -x", m),
		fmt.Sprintf("# Paste the %d decrypted share values when prompted", m),
		"# Reconstructed secret is printed to stdout",
	}
}

// CmdImportWrapKey returns commands to import the wrap key into the YubiHSM.
func CmdImportWrapKey(caName string) []string {
	return []string{
		"# Import wrap key into YubiHSM #1",
		"yubihsm-shell \\",
		"  --action put-wrap-key \\",
		`  --authkey 1 --password "password" \`,
		"  --object-id 0x0100 \\",
		fmt.Sprintf(`  --label "%s Wrap Key" \`, caName),
		`  --capabilities "export-wrapped,import-wrapped" \`,
		`  --delegated "sign-ecdsa,exportable-under-wrap" \`,
		`  --in <(echo -n "${WRAP_KEY}" | xxd -r -p)`,
		"",
		"# Repeat for YubiHSM #2 (redundancy)",
		"",
		"# Verify wrap key object is present on both devices",
		`yubihsm-shell --action list-objects --authkey 1 --password "password"`,
	}
}

// CmdSecureCleanup returns the cleanup commands to wipe all plaintext material.
func CmdSecureCleanup() []string {
	return []string{
		"# Securely wipe all ceremony working files",
		"shred -vzn 3 /tmp/ceremony/*",
		"rm -rf /tmp/ceremony",
		"",
		"# Clear shell history",
		"history -c",
		"",
		"# Unset all environment variables containing key material",
		"unset WRAP_KEY",
		`for i in $(seq 1 20); do unset C${i}_RECIPIENT; done`,
		"",
		`echo "Cleanup complete — power off the workstation now"`,
	}
}
