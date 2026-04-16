# go-root-ceremony

[![Go Reference](https://pkg.go.dev/badge/github.com/sirosfoundation/go-root-ceremony.svg)](https://pkg.go.dev/github.com/sirosfoundation/go-root-ceremony)
[![Go Report Card](https://goreportcard.com/badge/github.com/sirosfoundation/go-root-ceremony)](https://goreportcard.com/report/github.com/sirosfoundation/go-root-ceremony)
[![CI](https://github.com/sirosfoundation/go-root-ceremony/actions/workflows/ci.yml/badge.svg)](https://github.com/sirosfoundation/go-root-ceremony/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/sirosfoundation/go-root-ceremony)](go.mod)
[![License](https://img.shields.io/github/license/sirosfoundation/go-root-ceremony)](LICENSE)

Root CA key ceremony script generator. Produces self-contained, printable HTML ceremony scripts for performing high-security cryptographic operations using hardware security modules and Shamir Secret Sharing.

## Overview

`go-root-ceremony` generates step-by-step ceremony scripts that guide operators through secure key generation and management. The generated HTML documents are self-contained and printable — designed for air-gapped environments where digital documents may not be available.

### Supported Ceremony Types

| Type | Description |
|------|-------------|
| `root-ca-wrap` | Wrap key generation for Root CA |
| `root-ca-keygen` | HSM key generation for Root CA |
| `issuing-wrap` | Wrap key generation for Issuing CA |
| `recovery` | Key recovery ceremony |

### Security Features

- **Hardware Security Modules** — YubiHSM 2 FIPS for key storage
- **Hardware tokens** — YubiKey PIV enrollment for individual custodians
- **Shamir Secret Sharing** — Key material split across 3–10 custodians with configurable threshold
- **Age encryption** — Encrypted share storage using `age` and `age-plugin-yubikey`
- **Air-gap verification** — Network-down checks before any key material is handled
- **Secure workdir** — tmpfs-backed workspace to prevent key material hitting disk
- **Dual storage** — USB drives, printed QR codes, or both
- **Standards alignment** — EN 319 401 §7.5/7.6, TS 119 431-1

## Installation

```bash
go install github.com/sirosfoundation/go-root-ceremony@latest
```

Or build from source:

```bash
git clone git@github.com:sirosfoundation/go-root-ceremony.git
cd go-root-ceremony
make build
```

Cross-platform builds (Linux/macOS/Windows, amd64/arm64):

```bash
make build-all
```

## Usage

### Generate a config template

```bash
go-root-ceremony init -output ceremony.yaml
```

### Edit the config

```yaml
organization: "Siros Foundation"
ca_name: "SirosID Root CA G1"
location: "Secure Room, Stockholm"
date: "2026-04-16"
operator: "Jane Doe"
ceremony_type: "root-ca-wrap"
shamir:
  shares: 5
  threshold: 3
custodians:
  - name: "Alice"
  - name: "Bob"
  - name: "Carol"
  - name: "David"
  - name: "Eve"
witnesses:
  - name: "Frank"
  - name: "Grace"
options:
  include_verification: true
  include_yubihsm_import: true
  share_storage: "both"
```

### Generate the ceremony script

```bash
go-root-ceremony generate -config ceremony.yaml -output ceremony.html
```

### Interactive mode

```bash
go-root-ceremony generate -interactive -output ceremony.html
```

## Prerequisites

The generated ceremony scripts expect the following tools on the air-gapped ceremony machine:

- `yubihsm-shell` — YubiHSM 2 management
- `age` / `age-keygen` — Encryption
- `age-plugin-yubikey` — YubiKey-backed age identities
- `ssss-split` / `ssss-combine` — Shamir Secret Sharing
- `ykman` — YubiKey management
- `qrencode` — QR code generation (if using printed shares)

## Development

```bash
make test           # Run tests with race detector
make test-coverage  # Generate coverage report
make lint           # Run linter
make vet            # Run go vet
make fmt            # Format code
make check          # All of the above
```
