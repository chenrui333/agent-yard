package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func (a *App) newGCCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gc",
		Short: "Report cleanup candidates for yard run/review state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runGC()
		},
	}
}

func (a *App) runGC() error {
	for _, dir := range []string{a.yardPath("runs"), a.yardPath("reviews")} {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			a.printf("missing %s\n", dir)
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", dir, err)
		}
		a.printf("%s\n", dir)
		if len(entries) == 0 {
			a.printf("  no candidates\n")
			continue
		}
		for _, entry := range entries {
			a.printf("  %s\n", filepath.Join(dir, entry.Name()))
		}
	}
	a.printf("gc is report-only in the MVP; remove files manually after review.\n")
	return nil
}
