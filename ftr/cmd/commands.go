package cmd

// This file ensures all commands are properly imported and registered

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
