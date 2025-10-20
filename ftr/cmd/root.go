package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ftr",
	Short: "FtR - Package Manager, written by Quan Thai",
	Long: `FtR is a command-line tool for managing file repositories
and packages using the FSDL format. It integrates with InkDrop for file sharing.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// No mandatory sudo required. Commands that need elevated privileges
		// should perform their own checks.
	},
}

func Execute() error {
	return rootCmd.Execute()
}
