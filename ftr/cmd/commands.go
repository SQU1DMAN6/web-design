package cmd

// This file ensures all commands are properly imported and registered

import (
	_ "fmt"
	_ "io"
	_ "os"
	_ "path/filepath"
	_ "strings"
)

// Commands registers all available commands
func init() {
	rootCmd.AddCommand(
		getCmd,
		upCmd,
		clearCmd,
		removeCmd,
		loginCmd,
		logoutCmd,
		sessionCmd,
		packCmd,
		boxletCmd,
		initCmd,
		searchCmd,
		downCmd,
		buildCmd,
		versionCmd,
		listCmd,
		queryCmd,
		remoteCmd,
		upgradeCmd,
		automountCmd,
		unmountCmd,
	)
}
