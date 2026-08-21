package lethe_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"

	lethe "github.com/zyrophix/lethe"
)

// ExampleClean previews a risky clean without changing anything.
func ExampleClean() {
	res, err := lethe.Clean(context.Background(), lethe.Options{
		DryRun:  true,
		MaxRisk: lethe.RiskRisky,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("would clean %d artifacts\n", res.Cleaned)
}

// ExampleClean_full shows a real run with structured logging, backup and
// advanced options.
func ExampleClean_full() {
	var logBuf bytes.Buffer
	res, err := lethe.Clean(context.Background(), lethe.Options{
		DryRun:   false,
		MaxRisk:  lethe.RiskRisky,
		Logger:   lethe.NewTextLogger(&logBuf),
		AuditLog: nil,
		Advanced: &lethe.AdvancedOptions{
			Backup: &lethe.BackupOptions{Dir: "/tmp/lethe-backup"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("cleaned=%d failed=%d skipped=%d in %s\n",
		res.Cleaned, res.Failed, res.Skipped, res.Duration)
}

// ExampleVerifyResults checks leftovers and prints per-artifact status.
func ExampleVerifyResults() {
	results, err := lethe.VerifyResultsOpts(context.Background(), lethe.VerifyOptions{
		MaxRisk: lethe.RiskSafe,
		Logger:  lethe.NewJSONLogger(os.Stdout),
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range results {
		if !r.Cleaned {
			fmt.Printf("leftover: %s (%s)\n", r.Path, r.Reason)
		}
	}
}

// ExampleShredFile securely destroys one file with three overwrite passes.
func ExampleShredFile() {
	if err := lethe.ShredFile(context.Background(), "/tmp/secret.txt", 3); err != nil {
		log.Fatal(err)
	}
}
