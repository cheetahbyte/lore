package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	projectsync "github.com/cheetahbyte/lore/internal/sync"
	"github.com/spf13/cobra"
)

func newWatchCmd(app *App) *cobra.Command {
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "watch [path]",
		Short: "Keep project dependencies indexed while manifests change",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			if _, err := projectsync.Discover(cmd.Context(), root); err != nil {
				return err
			}
			last := manifestStamp(root)
			if err := runSync(cmd, app, root); err != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), err)
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-cmd.Context().Done():
					return cmd.Context().Err()
				case <-ticker.C:
					stamp := manifestStamp(root)
					if stamp == last {
						continue
					}
					last = stamp
					if err := runSync(cmd, app, root); err != nil {
						fmt.Fprintln(cmd.ErrOrStderr(), err)
					}
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Minute, "minimum time between manifest checks")
	return cmd
}

func manifestStamp(root string) time.Time {
	var latest time.Time
	for _, name := range []string{"package.json", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "go.mod", "go.sum", "requirements.txt", "pyproject.toml", "Cargo.toml", "Cargo.lock"} {
		if info, err := os.Stat(filepath.Join(root, name)); err == nil && info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}
	return latest
}
