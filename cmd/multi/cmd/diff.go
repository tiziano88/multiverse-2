package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tiziano88/multiverse/nodeservice"
)

var diffCmd = &cobra.Command{
	Use: "diff",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			fmt.Println("usage")
			fmt.Println("multi diff <from> <to>")
			fmt.Println("<from> and <to> are either local file paths, or multiverse hashes")
			os.Exit(1)
		}

		from := args[0]
		if !strings.HasPrefix(from, "baf") {
			hash, inMemory := buildInMemory(from)
			blobStore = nodeservice.Multiplex{
				Inner: []nodeservice.NodeService{
					inMemory,
					blobStore,
				},
			}
			from = hash.String()
		}

		to := args[1]
		if !strings.HasPrefix(to, "baf") {
			hash, inMemory := buildInMemory(to)
			blobStore = nodeservice.Multiplex{
				Inner: []nodeservice.NodeService{
					inMemory,
					blobStore,
				},
			}
			to = hash.String()
		}

		diff(from, to)
	},
}
