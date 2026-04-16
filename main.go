package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Version is injected at build time via ldflags.
var Version = "dev"

const usage = `go-root-ceremony — Root CA key ceremony script generator

Usage:
  go-root-ceremony generate [flags]   Generate a ceremony script
  go-root-ceremony init    [flags]    Write an example config file
  go-root-ceremony version            Print version

Generate flags:
  -config string    Path to YAML config file (required, or use -interactive)
  -output string    Output HTML file path (default: ceremony-script.html)
  -interactive      Prompt for all values interactively (ignores -config)

Init flags:
  -output string    Path to write example config (default: ceremony.yaml)

Examples:
  # Generate from config file
  go-root-ceremony generate -config ceremony.yaml -output script.html

  # Build config interactively then generate
  go-root-ceremony generate -interactive -output script.html

  # Write an example config to customise
  go-root-ceremony init -output my-ceremony.yaml
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		runGenerate(os.Args[2:])
	case "init":
		runInit(os.Args[2:])
	case "version":
		fmt.Printf("go-root-ceremony %s\n", Version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", os.Args[1], usage)
		os.Exit(1)
	}
}

func runGenerate(args []string) {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to YAML config file")
	outputPath := fs.String("output", "ceremony-script.html", "Output HTML file path")
	interactive := fs.Bool("interactive", false, "Prompt for all values interactively")
	_ = fs.Parse(args)

	var cfg Config
	var err error

	switch {
	case *interactive:
		cfg, err = promptConfig()
		if err != nil {
			fatalf("interactive config: %v", err)
		}
	case *configPath != "":
		cfg, err = LoadConfig(*configPath)
		if err != nil {
			fatalf("loading config: %v", err)
		}
	default:
		fatalf("either -config <file> or -interactive is required\n\n%s", usage)
	}

	html, err := Generate(&cfg)
	if err != nil {
		fatalf("generating script: %v", err)
	}

	if err := os.WriteFile(*outputPath, []byte(html), 0600); err != nil {
		fatalf("writing output: %v", err)
	}

	fmt.Printf("✓ Ceremony script written to: %s\n", *outputPath)
	fmt.Printf("  Organisation : %s\n", cfg.OrgDisplay())
	fmt.Printf("  CA           : %s\n", cfg.CADisplay())
	fmt.Printf("  Type         : %s\n", cfg.CeremonyType.Title())
	fmt.Printf("  SSS          : %d-of-%d\n", cfg.Shamir.Threshold, cfg.Shamir.Shares)
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	outputPath := fs.String("output", "ceremony.yaml", "Path to write example config")
	_ = fs.Parse(args)

	cfg := DefaultConfig()
	cfg.Organization = "Example Organisation AB"
	cfg.CAName = "Example Root CA G1"
	cfg.Location = "Secure Room, Building A"
	cfg.Operator = "Jane Doe"
	cfg.Custodians = []Person{
		{Name: "Alice Andersson"},
		{Name: "Bob Bergström"},
		{Name: "Charlie Carlsson"},
		{Name: "David Davidsson"},
		{Name: "Eve Eriksson"},
	}
	cfg.Witnesses = []Person{
		{Name: "Frank Franzén"},
		{Name: "Grace Gustafsson"},
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		fatalf("marshalling example config: %v", err)
	}

	header := "# go-root-ceremony configuration\n" +
		"# ceremony_type: root-ca-wrap | root-ca-keygen | issuing-wrap | recovery\n" +
		"# share_storage:  usb | print | both\n\n"

	if err := os.WriteFile(*outputPath, append([]byte(header), data...), 0600); err != nil {
		fatalf("writing config: %v", err)
	}
	fmt.Printf("✓ Example config written to: %s\n", *outputPath)
	fmt.Printf("  Edit the file and run: go-root-ceremony generate -config %s\n", *outputPath)
}

// ── Interactive prompt ─────────────────────────────────────────────────────

func promptConfig() (Config, error) {
	cfg := DefaultConfig()
	r := bufio.NewReader(os.Stdin)

	fmt.Println("\ngo-root-ceremony — Interactive Configuration")
	fmt.Println(strings.Repeat("─", 50))

	cfg.Organization = prompt(r, "Organisation name", "")
	cfg.CAName = prompt(r, "CA name", "Root CA G1")
	cfg.Location = prompt(r, "Ceremony location", "")
	cfg.Date = prompt(r, "Ceremony date (YYYY-MM-DD)", cfg.Date)
	cfg.Operator = prompt(r, "Ceremony operator name", "")

	fmt.Println("\nCeremony type:")
	fmt.Println("  1) root-ca-wrap    — Root CA wrap key generation")
	fmt.Println("  2) root-ca-keygen  — Root CA HSM key generation")
	fmt.Println("  3) issuing-wrap    — Issuing CA wrap key generation")
	fmt.Println("  4) recovery        — Key recovery ceremony")
	switch promptN(r, "Type [1-4]", 1, 1, 4) {
	case 1:
		cfg.CeremonyType = CeremonyRootCAWrap
	case 2:
		cfg.CeremonyType = CeremonyRootCAKeygen
	case 3:
		cfg.CeremonyType = CeremonyIssuingWrap
	case 4:
		cfg.CeremonyType = CeremonyRecovery
	}

	fmt.Println("\nShamir Secret Sharing parameters:")
	n := promptN(r, "Total shares (N)", 5, 3, 10)
	m := promptN(r, fmt.Sprintf("Threshold to reconstruct (M, max %d)", n-1), 3, 2, n-1)
	cfg.Shamir.Shares = n
	cfg.Shamir.Threshold = m

	fmt.Printf("\nCustodian names (%d custodians):\n", n)
	cfg.Custodians = make([]Person, n)
	for i := range cfg.Custodians {
		cfg.Custodians[i].Name = prompt(r, fmt.Sprintf("  Custodian %d", i+1), "")
	}

	numWitnesses := promptN(r, "\nNumber of witnesses", 2, 2, 5)
	cfg.Witnesses = make([]Person, numWitnesses)
	for i := range cfg.Witnesses {
		cfg.Witnesses[i].Name = prompt(r, fmt.Sprintf("  Witness %d", i+1), "")
	}

	fmt.Println("\nOptions:")
	cfg.Options.IncludeVerification = promptBool(r, "Include live verification step", true)
	cfg.Options.IncludeYubiHSMImport = promptBool(r, "Include YubiHSM wrap-key import", true)

	fmt.Println("Share storage method:")
	fmt.Println("  1) usb   — age-encrypted files on USB drives")
	fmt.Println("  2) print — printed QR codes")
	fmt.Println("  3) both  — USB drives and printed QR codes")
	switch promptN(r, "Storage [1-3]", 1, 1, 3) {
	case 1:
		cfg.Options.ShareStorage = StorageUSB
	case 2:
		cfg.Options.ShareStorage = StoragePrint
	case 3:
		cfg.Options.ShareStorage = StorageBoth
	}

	fmt.Println()
	return cfg, nil
}

func prompt(r *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptN(r *bufio.Reader, label string, def, min, max int) int {
	for {
		raw := prompt(r, fmt.Sprintf("%s (%d-%d)", label, min, max), strconv.Itoa(def))
		n, err := strconv.Atoi(raw)
		if err == nil && n >= min && n <= max {
			return n
		}
		fmt.Printf("  Please enter a number between %d and %d\n", min, max)
	}
}

func promptBool(r *bufio.Reader, label string, def bool) bool {
	defStr := "y"
	if !def {
		defStr = "n"
	}
	raw := prompt(r, label+" (y/n)", defStr)
	return strings.ToLower(strings.TrimSpace(raw)) != "n"
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
