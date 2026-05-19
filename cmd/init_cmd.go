package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initServerID string
var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a starter disform.yml",
	Long:  "Writes a disform.yml template to get you started. Use --server-id to pre-fill your server ID.",
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVar(&initServerID, "server-id", "", "Discord server ID to embed in the config")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing disform.yml")
}

const initTemplate = `server_id: %q

roles:
  admin:
    color: "#E74C3C"
    hoist: true
    mentionable: true
    permissions:
      - administrator

  moderator:
    color: "#E67E22"
    hoist: true
    permissions:
      - kick_members
      - manage_messages

  member:
    color: "#2ECC71"

categories:
  General:
    position: 0
    channels:
      welcome:
        type: text
        topic: "Welcome to the server!"
        permissions:
          "@everyone":
            deny:
              - send_messages
      general-chat:
        type: text
        topic: "General discussion"
      announcements:
        type: announcement

  Voice:
    position: 1
    channels:
      general-voice:
        type: voice
        bitrate: 64000
        user_limit: 0
      music:
        type: voice
        bitrate: 128000
        user_limit: 10
`

func runInit(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(configFile); err == nil && !initForce {
		return fmt.Errorf("%s already exists — use --force to overwrite", configFile)
	}

	serverID := initServerID
	if serverID == "" {
		serverID = "YOUR_SERVER_ID_HERE"
	}

	content := fmt.Sprintf(initTemplate, serverID)
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", configFile, err)
	}

	fmt.Printf("Created %s\n", configFile)
	if initServerID == "" {
		fmt.Println("  → Set server_id in the file, then run `disform plan`")
	} else {
		fmt.Println("  → Review the config, then run `disform plan`")
	}
	return nil
}
