package cli

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/zyrophix/lethe/internal/engine"
	"github.com/zyrophix/lethe/internal/module"
	"github.com/zyrophix/lethe/internal/modules/darwin"
	"github.com/zyrophix/lethe/internal/modules/linux"
	"github.com/zyrophix/lethe/internal/modules/windows"
	"github.com/zyrophix/lethe/internal/output"
	"github.com/zyrophix/lethe/internal/platform"
	"github.com/zyrophix/lethe/internal/risk"

	"github.com/spf13/cobra"
)

var (
	dryRun      bool
	maxRisk     string
	modulesFlag string
	parallel    bool
	doBackup    bool
	backupDir   string
	doShred     bool
	timestomp   bool
	wipeFree    bool
	stripXattr  bool
	outputFmt   string
	auditLog    string
	force       bool
	debug       bool
)

var rootCmd = &cobra.Command{
	Use:   "lethe",
	Short: "Anti-forensics trace cleaner",
	Long:  "Lethe — cross-platform anti-forensics trace cleaner with risk-gated operations.",
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean forensic traces",
	Long:  "Clean forensic traces from the system. Use --dry-run first to preview changes.",
	RunE:  runClean,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available modules",
	Long:  "List all available cleaning modules with their risk levels and artifact counts.",
	RunE:  runList,
}

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore from backup",
	Long:  "Restore artifacts from a previously created backup.",
	RunE:  runRestore,
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify traces were cleaned",
	Long:  "Verify that forensic traces have been cleaned. Exits 1 if any artifact was not cleaned.",
	RunE:  runVerify,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("lethe v0.1.0 %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	cleanCmd.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "preview changes without making them")
	cleanCmd.Flags().StringVarP(&maxRisk, "max-risk", "r", "risky", "max risk level: safe, risky, destructive")
	cleanCmd.Flags().StringVarP(&modulesFlag, "modules", "m", "", "comma-separated list of modules to run")
	cleanCmd.Flags().BoolVarP(&parallel, "parallel", "p", false, "run modules concurrently")
	cleanCmd.Flags().BoolVarP(&doBackup, "backup", "b", false, "backup artifacts before cleaning")
	cleanCmd.Flags().StringVar(&backupDir, "backup-dir", "", "override backup directory")
	cleanCmd.Flags().BoolVarP(&doShred, "shred", "s", false, "secure overwrite files before deletion")
	cleanCmd.Flags().BoolVar(&timestomp, "timestomp", false, "randomize file timestamps after truncate")
	cleanCmd.Flags().BoolVar(&wipeFree, "wipe-free-space", false, "fill free space to destroy deleted data")
	cleanCmd.Flags().BoolVar(&stripXattr, "strip-xattr", false, "remove extended attributes after cleaning")
	cleanCmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "output format: text, json")
	cleanCmd.Flags().StringVar(&auditLog, "audit-log", "", "write audit log to file")
	cleanCmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompts")
	cleanCmd.Flags().BoolVarP(&debug, "debug", "d", false, "verbose debug output")

	restoreCmd.Flags().StringVar(&backupDir, "backup-dir", "", "backup directory to restore from")

	verifyCmd.Flags().StringVarP(&modulesFlag, "modules", "m", "", "comma-separated list of modules to verify")
	verifyCmd.Flags().StringVarP(&outputFmt, "output", "o", "text", "output format: text, json")
	verifyCmd.Flags().StringVarP(&maxRisk, "max-risk", "r", "risky", "max risk level: safe, risky, destructive")

	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(versionCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func newRegistry() *module.Registry {
	r := module.NewRegistry()
	linux.RegisterAll(r)
	darwin.RegisterAll(r)
	windows.RegisterAll(r)
	return r
}

func newWriter() output.Writer {
	switch outputFmt {
	case "json":
		return output.NewJSONWriter(debug, dryRun, auditLog)
	default:
		return output.NewTextWriter(debug, auditLog)
	}
}

func runClean(cmd *cobra.Command, args []string) error {
	riskLevel, err := risk.ParseRiskLevel(maxRisk)
	if err != nil {
		return fmt.Errorf("invalid --max-risk: %w", err)
	}

	policy := risk.NewPolicy(riskLevel)
	registry := newRegistry()
	writer := newWriter()
	defer writer.Flush()

	platform := runtime.GOOS
	platformModules := registry.ListForPlatform(platform)

	if len(platformModules) == 0 {
		return fmt.Errorf("no modules available for platform %s", platform)
	}

	var selectedModules []string
	if modulesFlag != "" {
		selectedModules = strings.Split(modulesFlag, ",")
		for i, m := range selectedModules {
			selectedModules[i] = strings.TrimSpace(m)
		}
		for _, name := range selectedModules {
			if _, ok := registry.GetForPlatform(runtime.GOOS, name); !ok {
				return fmt.Errorf("unknown module: %s", name)
			}
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	if outputFmt == "json" {
		fmt.Fprintln(os.Stderr, "Lethe v0.1.0 — WARNING: This tool permanently deletes forensic traces!")
	} else {
		printBanner()
	}

	if !force && !dryRun {
		if !confirmAction(writer, policy, riskLevel, selectedModules, registry) {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if wipeFree && !force && !dryRun {
		fmt.Print("\033[31m  Wipe free space is destructive and slow. Type 'I UNDERSTAND' to proceed:\033[0m ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		if strings.TrimSpace(response) != "I UNDERSTAND" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	if dryRun {
		writer.Info("lethe", "DRY RUN mode — no changes will be made")
	}

	writer.Info("lethe", fmt.Sprintf("Platform: %s | Max risk: %s", platform, riskLevel))

	eng := engine.New(registry, policy, writer, homeDir)

	return eng.Run(engine.RunOptions{
		DryRun:        dryRun,
		UseShred:      doShred,
		Timestomp:     timestomp,
		WipeFreeSpace: wipeFree,
		StripXattr:    stripXattr,
		UseBackup:     doBackup,
		BackupDir:     backupDir,
		Parallel:      parallel,
		Force:         force,
		ModuleNames:   selectedModules,
		Debug:         debug,
	})
}

func runList(cmd *cobra.Command, args []string) error {
	registry := newRegistry()
	platform := runtime.GOOS
	mods := registry.ListForPlatform(platform)

	fmt.Printf("Available modules for %s:\n\n", platform)
	for _, m := range mods {
		artCount := len(m.Artifacts())
		riskStr := m.Risk().String()
		switch m.Risk() {
		case risk.RiskSafe:
			fmt.Printf("  %-14s %-12s %d artifacts\n", m.Name(), riskStr, artCount)
		case risk.RiskRisky:
			fmt.Printf("  %-14s \033[33m%-12s\033[0m %d artifacts\n", m.Name(), riskStr, artCount)
		case risk.RiskDestructive:
			fmt.Printf("  %-14s \033[31m%-12s\033[0m %d artifacts\n", m.Name(), riskStr, artCount)
		}
	}
	fmt.Println()
	return nil
}

func runVerify(cmd *cobra.Command, args []string) error {
	riskLevel, err := risk.ParseRiskLevel(maxRisk)
	if err != nil {
		return fmt.Errorf("invalid --max-risk: %w", err)
	}

	policy := risk.NewPolicy(riskLevel)
	registry := newRegistry()

	mods := registry.ListForPlatform(runtime.GOOS)
	if modulesFlag != "" {
		var filtered []module.Module
		for _, name := range strings.Split(modulesFlag, ",") {
			m, ok := registry.GetForPlatform(runtime.GOOS, strings.TrimSpace(name))
			if !ok {
				return fmt.Errorf("unknown module: %s", strings.TrimSpace(name))
			}
			filtered = append(filtered, m)
		}
		mods = filtered
	}

	var artifacts []module.Artifact
	for _, m := range mods {
		all := m.Artifacts()
		for i := range all {
			if policy.Allowed(all[i].GetRisk()) {
				artifacts = append(artifacts, all[i])
			}
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	results := engine.VerifyAll(artifacts, platform.UserHomes(homeDir))

	var uncleaned []engine.VerifyResult
	writer := newWriter()
	defer writer.Flush()

	if outputFmt == "json" {
		for _, r := range results {
			status := "cleaned"
			if !r.Cleaned {
				status = "not_cleaned"
				uncleaned = append(uncleaned, r)
			}
			fmt.Printf(`{"artifact":%q,"status":%q,"reason":%q}`+"\n", r.Artifact.Path, status, r.Reason)
		}
		fmt.Printf(`{"total":%d,"cleaned":%d,"not_cleaned":%d}`+"\n", len(results), len(results)-len(uncleaned), len(uncleaned))
	} else {
		for _, r := range results {
			status := "CLEANED"
			if !r.Cleaned {
				status = "NOT CLEANED"
				uncleaned = append(uncleaned, r)
			}
			fmt.Printf("%-13s %-12s %s\n", status, r.Artifact.Path, r.Reason)
		}
		fmt.Printf("\n%d cleaned, %d not cleaned\n", len(results)-len(uncleaned), len(uncleaned))
	}

	if len(uncleaned) > 0 {
		return fmt.Errorf("verification failed: %d artifacts not cleaned", len(uncleaned))
	}
	return nil
}

func runRestore(cmd *cobra.Command, args []string) error {
	if backupDir == "" {
		return fmt.Errorf("--backup-dir is required for restore")
	}
	b := engine.NewBackup(backupDir)
	if err := b.Restore(); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}
	fmt.Println("Restore complete.")
	return nil
}

func printBanner() {
	fmt.Println("================================================")
	fmt.Println("  Lethe — Anti-Forensics Trace Cleaner v0.1.0")
	fmt.Println("================================================")
	fmt.Println()
	fmt.Println("  WARNING: This tool will permanently delete forensic traces!")
	fmt.Println("  This action cannot be undone and may impact system stability.")
	fmt.Println()
}

func confirmAction(w output.Writer, policy risk.Policy, maxRisk risk.RiskLevel, selected []string, registry *module.Registry) bool {
	var destructiveArtifacts []string

	mods := registry.ListForPlatform(runtime.GOOS)
	if len(selected) > 0 {
		var filtered []module.Module
		for _, name := range selected {
			if m, ok := registry.GetForPlatform(runtime.GOOS, name); ok {
				filtered = append(filtered, m)
			}
		}
		mods = filtered
	}

	for _, m := range mods {
		for _, a := range m.Artifacts() {
			if a.GetRisk() == risk.RiskDestructive && policy.Allowed(risk.RiskDestructive) {
				desc := a.Description
				if desc == "" {
					desc = a.Path
				}
				destructiveArtifacts = append(destructiveArtifacts, desc)
			}
		}
	}

	if len(destructiveArtifacts) > 0 {
		fmt.Println("\033[31m  DESTRUCTIVE operations detected:\033[0m")
		for _, d := range destructiveArtifacts {
			fmt.Printf("    - %s\n", d)
		}
		fmt.Println()
		fmt.Print("Type 'I UNDERSTAND' to proceed: ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)
		if response != "I UNDERSTAND" {
			return false
		}
	} else {
		fmt.Print("Do you want to continue? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(response)), "y") {
			return false
		}
	}

	return true
}
