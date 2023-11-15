package models

import (
	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/iota.go/v4/hexutil"
	"github.com/iotaledger/wasp/packages/parameters"
)

type NodeOwnerCertificateResponse struct {
	Certificate string `json:"certificate" swagger:"desc(Certificate stating the ownership. (Hex)),required"`
}

// Storage defines the parameters of rent cost calculations on objects which take node resources.
type Storage struct {
	// Defines the rent of a single virtual byte denoted in IOTA tokens.
	StorageCost string `json:"storageCost" swagger:"desc(The storage cost),required"`
	// Defines the factor to be used for data only fields.
	FactorData byte `json:"factorData" swagger:"desc(The factor for data fields),required"`
}

type ProtocolParameters struct {
	// The version of the protocol running.
	Version []byte `json:"version" swagger:"desc(The protocol version),required"`
	// The human friendly name of the network.
	NetworkName string `json:"networkName" swagger:"desc(The network name),required"`
	// The HRP prefix used for Bech32 addresses in the network.
	Bech32HRP iotago.NetworkPrefix `json:"bech32Hrp" swagger:"desc(The human readable network prefix),required"`
	// The minimum pow score of the network.
	// MinPoWScore uint32 `json:"minPowScore" swagger:"desc(The minimal PoW score),required,min(1)"`
	// The below max depth parameter of the network.
	BelowMaxDepth uint8 `json:"belowMaxDepth" swagger:"desc(The networks max depth),required,min(1)"`
	// The rent structure used by given node/network.
	Storage Storage `json:"rentStructure" swagger:"desc(The rent structure of the protocol),required"`
	// TokenSupply defines the current token supply on the network.
	TokenSupply string `json:"tokenSupply" swagger:"desc(The token supply),required"`
}

type L1Params struct {
	MaxPayloadSize int                      `json:"maxPayloadSize" swagger:"desc(The max payload size),required"`
	Protocol       ProtocolParameters       `json:"protocol" swagger:"desc(The protocol parameters),required"`
	BaseToken      parameters.BaseTokenInfo `json:"baseToken" swagger:"desc(The base token parameters),required"`
}

func MapL1Params(l1 *parameters.L1Params) *L1Params {
	// TODO: <lmoe> L1Params is almost completely outdated now and requires some time

	version, err := l1.Protocol.Version().Bytes()
	if err != nil {
		panic(err)
	}

	params := &L1Params{
		// There are no limits on how big from a size perspective an essence can be, so it is just derived from 32KB - Message fields without payload = max size of the payload
		MaxPayloadSize: 0xC0FF33,
		Protocol: ProtocolParameters{
			Version:     version,
			NetworkName: l1.Protocol.NetworkName(),
			Bech32HRP:   l1.Protocol.Bech32HRP(),
			Storage: Storage{
				StorageCost: hexutil.EncodeUint64(uint64(l1.Protocol.StorageScoreParameters().StorageCost)),
				FactorData:  byte(l1.Protocol.StorageScoreParameters().FactorData),
			},
			TokenSupply: hexutil.EncodeUint64(uint64(l1.Protocol.TokenSupply())),
		},
		BaseToken: parameters.BaseTokenInfo{
			Name:            l1.BaseToken.Name,
			TickerSymbol:    l1.BaseToken.TickerSymbol,
			Unit:            l1.BaseToken.Unit,
			Subunit:         l1.BaseToken.Subunit,
			Decimals:        l1.BaseToken.Decimals,
			UseMetricPrefix: l1.BaseToken.UseMetricPrefix,
		},
	}

	return params
}

type VersionResponse struct {
	Version string `json:"version" swagger:"desc(The version of the node),required"`
}

type InfoResponse struct {
	Version    string    `json:"version" swagger:"desc(The version of the node),required"`
	PublicKey  string    `json:"publicKey" swagger:"desc(The public key of the node (Hex)),required"`
	PeeringURL string    `json:"peeringURL" swagger:"desc(The net id of the node),required"`
	L1Params   *L1Params `json:"l1Params" swagger:"desc(The L1 parameters),required"`
}
