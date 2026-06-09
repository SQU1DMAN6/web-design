package cmd

import (
	"fmt"
	"ftr/pkg/api"
	"strings"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search Drops on the server",
	Long:  "Search for Drops by name or description",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.Join(args, " ")
		client, err := api.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}

		matches, err := client.SearchDrops(query)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		if len(matches) == 0 {
			fmt.Println("No matches found.")
			return nil
		}

		fmt.Println("Matches:")
		for _, m := range matches {
			user := m["user"]
			drop := m["repo"]
			desc := m["description"]
			if desc == "" {
				desc = "(no description)"
			}
			fmt.Printf("%s/%-20s %s\n", user, drop, desc)
		}
		return nil
	},
}
