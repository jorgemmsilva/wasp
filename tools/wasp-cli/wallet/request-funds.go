package wallet

import (
	"github.com/spf13/cobra"

	"github.com/iotaledger/wasp/tools/wasp-cli/cli/cliclients"
	"github.com/iotaledger/wasp/tools/wasp-cli/cli/wallet"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
)

func initRequestFundsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "request-funds",
		Short: "Request funds from the faucet",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			wallet := wallet.Load()
			log.Check(cliclients.L1Client().RequestFunds(wallet))

			model := &RequestFundsModel{
				Address: wallet.Address().Bech32(cliclients.API().ProtocolParameters().Bech32HRP()),
				Message: "success",
			}

			log.PrintCLIOutput(model)
		},
	}
}

type RequestFundsModel struct {
	Address string
	Message string
}

var _ log.CLIOutput = &RequestFundsModel{}

func (r *RequestFundsModel) AsText() (string, error) {
	template := `Request funds for address {{ .Address }} success`
	return log.ParseCLIOutputTemplate(r, template)
}
