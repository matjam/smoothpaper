package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/matjam/smoothpaper/internal/ipc"
	"github.com/spf13/cobra"
	"github.com/tidwall/pretty"
)

func NewStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Get smoothpaper status",
		Long:  `Returns the current status of the smoothpaper process.`,
		Run: func(cmd *cobra.Command, args []string) {
			response, err := ipc.SendStatus()
			if err != nil {
				log.Errorf("Error sending command: %v", err)
				return
			}

			PrintJSONColored(response)
		},
	}
}

func PrintJSONColored(data interface{}) {
	j, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Errorf("Error marshalling JSON: %v", err)
		return
	}

	jPretty := pretty.Color(j, nil)
	fmt.Println(string(jPretty))
}
