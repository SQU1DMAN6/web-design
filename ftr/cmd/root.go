package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ftr",
	Short: "FtR - Package Manager, written by Quan Thai",
	Long: `FtR is a command-line tool for managing file repositories
and packages using the FSDL format. It integrates with InkDrop for file sharing.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip root check for help and completion commands
		if cmd.Name() != "help" && cmd.Name() != "completion" {
			if os.Geteuid() != 0 {
				fmt.Println("Please run with sudo.")
				os.Exit(1)
			}
		}
	},
}

func Execute() error {
	return rootCmd.Execute()
}
