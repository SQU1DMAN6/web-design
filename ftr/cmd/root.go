package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "ftr",
	Short: "FtR - File Transfer and Repository Manager",
	Long: `FtR (File Transfer) is a command-line tool for managing file repositories
and packages using the FSDL format. It integrates with InkDrop for file sharing.

Commands:
  get [user/repo]        Download and install a package
  up [source] [repo]     Upload files to create a package
  r, remove [repo]       Remove an installed package
  clear                  Clean temporary directory
  login                  Log in to your account`,
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