package cmd

import (
	"fmt"
	"strings"

	"bazil.org/fuse"
	"github.com/spf13/cobra"
)

var unmountCmd = &cobra.Command{
	Use:   "unmount <mountpoint>",
	Short: "Unmount a mounted FtR Drop",
	Long:  "Unmount a local mount point created by the FtR mount command.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mountPoint := strings.TrimSpace(args[0])
		if mountPoint == "" {
			return fmt.Errorf("mount point is required")
		}

		if err := fuse.Unmount(mountPoint); err != nil {
			return fmt.Errorf("failed to unmount %s: %w", mountPoint, err)
		}

		fmt.Printf("Unmounted %s\n", mountPoint)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(unmountCmd)
}
