package setup

import (
	"github.com/spf13/cobra"

	"github.com/iotaledger/wasp/tools/wasp-cli/cli/config"
)

func Init(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVarP(&config.ConfigPath, "config", "c", "wasp-cli.json", "path to wasp-cli.json")
	rootCmd.PersistentFlags().BoolVarP(&config.WaitForCompletion, "wait", "w", true, "wait for request completion")

	rootCmd.AddCommand(initCheckVersionsCmd())
	rootCmd.AddCommand(initConfigSetCmd())
}
