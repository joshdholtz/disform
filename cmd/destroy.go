package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/joshholtz/disform/internal/discord"
	"github.com/joshholtz/disform/internal/state"
	"github.com/spf13/cobra"
)

var destroyAutoApprove bool

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy all managed Discord resources",
	Long:  "Deletes all resources tracked in state from the Discord server and clears the state file.",
	RunE:  runDestroy,
}

func init() {
	destroyCmd.Flags().BoolVar(&destroyAutoApprove, "auto-approve", false, "Skip confirmation prompt")
}

func runDestroy(cmd *cobra.Command, args []string) error {
	tok := getToken()
	if tok == "" {
		return fmt.Errorf("Discord bot token is required (set --token or DISCORD_TOKEN env var)")
	}

	st, err := state.LoadState(stateFile)
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	lock, err := state.AcquireLock(stateFile)
	if err != nil {
		return err
	}
	defer lock.Release()

	if len(st.Channels)+len(st.Categories)+len(st.Roles) == 0 {
		fmt.Println("No managed resources to destroy.")
		return nil
	}

	fmt.Printf("This will destroy %d channel(s), %d category/categories, and %d role(s).\n",
		len(st.Channels), len(st.Categories), len(st.Roles))

	if !destroyAutoApprove {
		fmt.Print("Are you sure you want to destroy ALL managed resources? (yes/no): ")
		reader := bufio.NewReader(os.Stdin)
		answer, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "yes" {
			fmt.Println("Destroy cancelled.")
			return nil
		}
	}

	client := discord.NewHTTPClient(tok)
	serverID := st.ServerID

	fmt.Println()

	// Delete channels first (sorted: channels must go before their parent category).
	for _, key := range sortedKeys(st.Channels) {
		rs := st.Channels[key]
		fmt.Printf("  Deleting channel %q (%s)... ", key, rs.DiscordID)
		if err := client.DeleteChannel(rs.DiscordID); err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Println("done")
		}
	}

	// Delete categories next.
	for _, name := range sortedKeys(st.Categories) {
		rs := st.Categories[name]
		fmt.Printf("  Deleting category %q (%s)... ", name, rs.DiscordID)
		if err := client.DeleteChannel(rs.DiscordID); err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Println("done")
		}
	}

	// Delete roles last.
	for _, name := range sortedKeys(st.Roles) {
		rs := st.Roles[name]
		fmt.Printf("  Deleting role %q (%s)... ", name, rs.DiscordID)
		if err := client.DeleteRole(serverID, rs.DiscordID); err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Println("done")
		}
	}

	// Clear state.
	newState := state.NewState(serverID)
	if err := state.SaveState(newState, stateFile); err != nil {
		return fmt.Errorf("clearing state file: %w", err)
	}

	fmt.Println("\nDestroy complete. State file cleared.")
	return nil
}
