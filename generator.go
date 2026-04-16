package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"strings"
	"time"
)

//go:embed templates/*.html.tmpl
var templateFS embed.FS

// templateData is passed to the ceremony HTML template.
type templateData struct {
	Config      *Config
	GeneratedAt string

	// Pre-built command blocks
	AirGapCmd          []string
	PrepWorkdirCmd     []string
	VerifySoftwareCmd  []string
	VerifyYubiHSMCmd   []string
	EnrollCmds         [][]string
	GenerateKeyCmd     []string
	GenerateHSMKeyCmd  []string
	SSSSplitCmd        []string
	EncryptCmds        [][]string
	VerifyReconstructCmd []string
	ImportWrapKeyCmd   []string
	CleanupCmd         []string
	RecoveryDecryptCmds [][]string
	RecoveryCombineCmd []string

	// Derived flags
	IsRecovery   bool
	IsHSMKeygen  bool
	IncludeUSBDrives bool
	StorageNote  string

	// Recovery ceremony: indices of custodians who decrypt
	RecoveryCustodians []int

	// Section numbers (vary based on options)
	CleanupSectionNum int
	RecordSectionNum  int
	SigSectionNum     int
}

// CeremonyDescription returns the descriptive paragraph for the ceremony type.
func (d *templateData) CeremonyDescription() string {
	return d.Config.CeremonyType.Description(
		d.Config.Shamir.Shares,
		d.Config.Shamir.Threshold,
	)
}

// Generate produces a complete self-contained HTML ceremony script.
func Generate(cfg *Config) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", fmt.Errorf("config validation: %w", err)
	}

	n := cfg.Shamir.Shares
	m := cfg.Shamir.Threshold

	isRecovery := cfg.CeremonyType == CeremonyRecovery
	isKeygen := cfg.CeremonyType == CeremonyRootCAKeygen
	includeUSB := cfg.Options.ShareStorage == StorageUSB || cfg.Options.ShareStorage == StorageBoth

	// Build per-custodian command slices
	enrollCmds := make([][]string, n)
	encryptCmds := make([][]string, n)
	for i := 0; i < n; i++ {
		enrollCmds[i] = CmdEnrollYubiKey(i)
		encryptCmds[i] = CmdEncryptShare(i, cfg.Options.ShareStorage)
	}

	// Recovery custodians: first M entries
	recoveryCustodians := make([]int, m)
	recoveryDecryptCmds := make([][]string, n)
	for i := 0; i < m; i++ {
		recoveryCustodians[i] = i
	}
	for i := 0; i < n; i++ {
		recoveryDecryptCmds[i] = CmdRecoveryDecrypt(i)
	}

	// Calculate dynamic section numbers
	// Sections: 1 Purpose, 2 Personnel, 3 Checklist, 4 Env, 5 Enroll,
	//           6 Generate, 7 Split, 8 Encrypt, 9 Verify (optional), 10 Cleanup, 11 Record, 12 Sig
	// Recovery path has fewer sections
	var cleanupNum, recordNum, sigNum int
	if isRecovery {
		cleanupNum = 7
		recordNum = 8
		sigNum = 9
	} else if cfg.Options.IncludeVerification {
		cleanupNum = 10
		recordNum = 11
		sigNum = 12
	} else {
		cleanupNum = 9
		recordNum = 10
		sigNum = 11
	}

	var genKeyCmd []string
	if isKeygen {
		genKeyCmd = CmdGenerateYubiHSMWrapKey(cfg.CADisplay())
	} else {
		genKeyCmd = CmdGenerateWrapKey()
	}

	d := &templateData{
		Config:               cfg,
		GeneratedAt:          time.Now().UTC().Format("2006-01-02 15:04:05"),
		AirGapCmd:            CmdVerifyAirGap(),
		PrepWorkdirCmd:       CmdPrepareWorkdir(),
		VerifySoftwareCmd:    CmdVerifySoftware(cfg.Options.IncludeYubiHSMImport),
		VerifyYubiHSMCmd:     CmdVerifyYubiHSM(),
		EnrollCmds:           enrollCmds,
		GenerateKeyCmd:       genKeyCmd,
		GenerateHSMKeyCmd:    CmdGenerateYubiHSMKey(cfg.CADisplay()),
		SSSSplitCmd:          CmdSSSplit(n, m),
		EncryptCmds:          encryptCmds,
		VerifyReconstructCmd: CmdVerifyReconstruct(m),
		ImportWrapKeyCmd:     CmdImportWrapKey(cfg.CADisplay()),
		CleanupCmd:           CmdSecureCleanup(),
		RecoveryDecryptCmds:  recoveryDecryptCmds,
		RecoveryCombineCmd:   CmdRecoveryCombine(m),
		IsRecovery:           isRecovery,
		IsHSMKeygen:          isKeygen,
		IncludeUSBDrives:     includeUSB,
		StorageNote:          cfg.Options.ShareStorage.Note(),
		RecoveryCustodians:   recoveryCustodians,
		CleanupSectionNum:    cleanupNum,
		RecordSectionNum:     recordNum,
		SigSectionNum:        sigNum,
	}

	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"not": func(b bool) bool { return !b },
		"codeblock": func(lines []string) template.HTML {
			return renderCodeBlock(lines)
		},
	}

	tmplContent, err := templateFS.ReadFile("templates/ceremony.html.tmpl")
	if err != nil {
		return "", fmt.Errorf("reading template: %w", err)
	}

	tmpl, err := template.New("ceremony").Funcs(funcMap).Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

// renderCodeBlock converts a []string of command lines into styled HTML.
// Lines starting with '#' get the comment style class.
func renderCodeBlock(lines []string) template.HTML {
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<pre>")
	for i, line := range lines {
		if i > 0 {
			sb.WriteString("\n")
		}
		escaped := template.HTMLEscapeString(line)
		if strings.HasPrefix(line, "#") {
			sb.WriteString(`<span class="comment">`)
			sb.WriteString(escaped)
			sb.WriteString(`</span>`)
		} else {
			sb.WriteString(escaped)
		}
	}
	sb.WriteString("</pre>")
	return template.HTML(sb.String())
}
