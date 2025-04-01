package cmd

import (
	"encoding/json"

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
			response, err := ipc.SendCommand(ipc.Command{
				Type: ipc.CommandStatus,
			})
			if err != nil {
				log.Errorf("Error sending command: %v", err)
				return
			}

			PrintJSONColored(response.Data)
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
	log.Info(string(jPretty))
}
