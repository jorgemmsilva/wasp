// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

package peering

import (
	"github.com/iotaledger/wasp/tools/wasp-cli/config"
	"github.com/iotaledger/wasp/tools/wasp-cli/log"
	"github.com/spf13/cobra"
)

var listTrustedCmd = &cobra.Command{
	Use:   "list-trusted",
	Short: "List trusted wasp nodes.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		trustedList, err := config.WaspClient().GetPeeringTrustedList()
		log.Check(err)
		header := []string{"PubKey", "NetID"}
		rows := make([][]string, len(trustedList))
		for i := range rows {
			rows[i] = []string{
				trustedList[i].PubKey,
				trustedList[i].NetID,
			}
		}
		log.PrintTable(header, rows)
	},
}
