package wallet

import (
	"github.com/spf13/cobra"
)

func initSendFundsCmd() *cobra.Command {
	var adjustStorageDeposit bool

	cmd := &cobra.Command{
		Use:   "send-funds <target-address> <token-id>:<amount> <token-id2>:<amount> ...",
		Short: "Transfer L1 tokens",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			panic("TODO rewrite")
			// _, targetAddress, err := iotago.ParseBech32(args[0])
			// log.Check(err)

			// tokens := util.ParseFungibleTokens(util.ArgsToFungibleTokensStr(args[1:]))
			// log.Check(err)

			// log.Printf("\nSending \n\t%v \n\tto: %v\n\n", tokens, args[0])

			// myWallet := wallet.Load()
			// senderAddress := myWallet.Address()
			// client := cliclients.L1Client()

			// outputSet, err := client.OutputMap(senderAddress)
			// log.Check(err)

			// if !adjustStorageDeposit {
			// 	// check if the resulting output needs to be adjusted for Storage Deposit
			// 	output := transaction.MakeBasicOutput(
			// 		targetAddress,
			// 		senderAddress,
			// 		&tokens.FungibleTokens,
			// 		0, // TODO is this correct?
			// 		nil,
			// 		[]iotago.UnlockCondition{},
			// 	)
			// 	util.SDAdjustmentPrompt(output)
			// }

			// tx, err := transaction.NewTransferTransaction(
			// 	&tokens.FungibleTokens,
			// 	0,
			// 	senderAddress,
			// 	myWallet.KeyPair,
			// 	targetAddress,
			// 	outputSet,
			// 	[]iotago.UnlockCondition{},
			// 	cliclients.API().TimeProvider().SlotFromTime(time.Now()),
			// 	false,
			// 	cliclients.L1Client().APIProvider(),
			// )
			// log.Check(err)

			// txID, err := tx.ID()
			// log.Check(err)

			// _, err = client.PostTxAndWaitUntilConfirmation(tx)
			// log.Check(err)

			// log.Printf("Transaction [%v] sent successfully.\n", txID.ToHex())
		},
	}

	cmd.Flags().BoolVarP(&adjustStorageDeposit, "adjust-storage-deposit", "s", false, "adjusts the amount of base tokens sent, if it's lower than the min storage deposit required")

	return cmd
}
