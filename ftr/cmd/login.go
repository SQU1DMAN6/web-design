package cmd

import (
	"bufio"
	"fmt"
	"github.com/spf13/cobra"
	"ftr/pkg/api"
	"os"
	"strings"
	"syscall"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to your account",
	Long: `Log in to your account to access remote repositories.
Required for uploading packages with 'up' command.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		// Get username
		fmt.Print("Username: ")
		username, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read username: %w", err)
		}
		username = strings.TrimSpace(username)

		// Get password securely
		fmt.Print("Password: ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println()

		// Create API client and login
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		if err := client.Login(username, string(password)); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		fmt.Println("Successfully logged in")
		return nil
	},
}