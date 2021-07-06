package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

var pullCmd = &cobra.Command{
	Use:  "pull",
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("pull")
	},
}
