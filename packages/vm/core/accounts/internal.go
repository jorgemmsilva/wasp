package accounts

import (
	"errors"
	"fmt"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv"
	"github.com/iotaledger/wasp/packages/kv/codec"
	"github.com/iotaledger/wasp/packages/kv/collections"
	"github.com/iotaledger/wasp/packages/kv/dict"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/errors/coreerrors"
)

var (
	ErrNotEnoughFunds                       = coreerrors.Register("not enough funds").Create()
	ErrNotEnoughBaseTokensForStorageDeposit = coreerrors.Register("not enough base tokens for storage deposit").Create()
	ErrNotEnoughAllowance                   = coreerrors.Register("not enough allowance").Create()
	ErrBadAmount                            = coreerrors.Register("bad native asset amount").Create()
	ErrRepeatingFoundrySerialNumber         = coreerrors.Register("repeating serial number of the foundry").Create()
	ErrFoundryNotFound                      = coreerrors.Register("foundry not found").Create()
	ErrOverflow                             = coreerrors.Register("overflow in token arithmetics").Create()
	ErrTooManyNFTsInAllowance               = coreerrors.Register("expected at most 1 NFT in allowance").Create()
	ErrNFTIDNotFound                        = coreerrors.Register("NFTID not found").Create()
)

const (
	// keyAllAccounts stores a map of <agentID> => true
	// Covered in: TestFoundries
	keyAllAccounts = "a"

	// prefixBaseTokens | <accountID> stores the amount of base tokens (big.Int)
	// Covered in: TestFoundries
	prefixBaseTokens = "b"
	// PrefixNativeTokens | <accountID> stores a map of <nativeTokenID> => big.Int
	// Covered in: TestFoundries
	// TODO [iota2.0]: needs migration if NativeTokenIDs are not preserved
	PrefixNativeTokens = "t"

	// L2TotalsAccount is the special <accountID> storing the total fungible tokens
	// controlled by the chain
	// Covered in: TestFoundries
	L2TotalsAccount = "*"

	// PrefixNFTs | <agentID> stores a map of <NFTID> => true
	// Covered in: TestDepositNFTWithMinStorageDeposit
	// TODO [iota2.0]: needs migration if NFTIDs are not preserved
	PrefixNFTs = "n"
	// PrefixNFTsByCollection | <agentID> | <collectionID> stores a map of <NFTID> => true
	// Covered in: TestNFTMint
	// Covered in: TestDepositNFTWithMinStorageDeposit
	// TODO [iota2.0]: needs migration if NFTIDs are not preserved
	PrefixNFTsByCollection = "c"
	// prefixNewlyMintedNFTs stores a map of <position in minted list> => <mintedNFTRecord> to be updated when the outputID is known
	// Covered in: TestNFTMint
	// TODO [iota2.0]: needs migration
	prefixNewlyMintedNFTs = "N"
	// prefixMintIDMap stores a map of <mintID> => <NFTID> it is updated when the NFTID of newly minted nfts is known
	// Covered in: TestNFTMint
	// TODO [iota2.0]: migrate mintID?
	// TODO [iota2.0]: needs migration if NFTIDs are not preserved
	prefixMintIDMap = "M"
	// PrefixFoundries + <agentID> stores a map of <foundrySN> (uint32) => true
	// Covered in: TestFoundries
	PrefixFoundries = "f"

	// noCollection is the special <collectionID> used for storing NFTs that do not belong in a collection
	// Covered in: TestNFTMint
	noCollection = "-"

	// keyNonce stores a map of <agentID> => nonce (uint64)
	// Covered in: TestNFTMint
	keyNonce = "m"

	// keyNativeTokenOutputMap stores a map of <nativeTokenID> => nativeTokenOutputRec
	// TODO [iota2.0]:
	//  - Does NativeTokenID need migration? Size is the same (38)
	//  - migrate nativeTokenOutputRec
	// Covered in: TestFoundries
	keyNativeTokenOutputMap = "TO"
	// keyFoundryOutputRecords stores a map of <foundrySN> => foundryOutputRec
	// Covered in: TestFoundries
	// TODO [iota2.0]: migrate foundryOutputRec
	keyFoundryOutputRecords = "FO"
	// keyNFTOutputRecords stores a map of <NFTID> => NFTOutputRec
	// Covered in: TestDepositNFTWithMinStorageDeposit
	// TODO [iota2.0]: needs migration if NFTIDs are not preserved
	// TODO [iota2.0]: migrate NFTOutputRec
	keyNFTOutputRecords = "NO"
	// keyNFTOwner stores a map of <NFTID> => isc.AgentID
	// Covered in: TestDepositNFTWithMinStorageDeposit
	// TODO [iota2.0]: needs migration if NFTIDs are not preserved
	keyNFTOwner = "NW"

	// keyNewNativeTokens stores an array of <nativeTokenID>, containing the newly created native tokens that need filling out the OutputID
	// Covered in: TestFoundries
	// TODO [iota2.0]: needs migration if NativeTokenIDs are not preserved
	keyNewNativeTokens = "TN"
	// keyNewFoundries stores an array of <foundrySN>, containing the newly created foundries that need filling out the OutputID
	// Covered in: TestFoundries
	keyNewFoundries = "FN"
	// keyNewNFTs stores an array of <NFTID>, containing the newly created NFTs that need filling out the OutputID
	// Covered in: TestDepositNFTWithMinStorageDeposit
	// TODO [iota2.0]: needs migration if NFTIDs are not preserved
	keyNewNFTs = "NN"
)

func (s *StateReader) accountKey(agentID isc.AgentID) kv.Key {
	if agentID.BelongsToChain(s.ctx.ChainID()) {
		// save bytes by skipping the chainID bytes on agentIDs for this chain
		return kv.Key(agentID.BytesWithoutChainID())
	}
	return kv.Key(agentID.Bytes())
}

func AllAccountsMap(contractState kv.KVStore) *collections.Map {
	return collections.NewMap(contractState, keyAllAccounts)
}

func (s *StateWriter) AllAccountsMap() *collections.Map {
	return AllAccountsMap(s.state)
}

func (s *StateReader) AllAccountsMap() *collections.ImmutableMap {
	return collections.NewMapReadOnly(s.state, keyAllAccounts)
}

func (s *StateReader) AccountExists(agentID isc.AgentID) bool {
	return s.AllAccountsMap().HasAt([]byte(s.accountKey(agentID)))
}

func (s *StateReader) AllAccountsAsDict() dict.Dict {
	ret := dict.New()
	s.AllAccountsMap().IterateKeys(func(accKey []byte) bool {
		ret.Set(kv.Key(accKey), []byte{0x01})
		return true
	})
	return ret
}

// touchAccount ensures the account is in the list of all accounts
func (s *StateWriter) touchAccount(agentID isc.AgentID) {
	s.AllAccountsMap().SetAt([]byte(s.accountKey(agentID)), codec.Bool.Encode(true))
}

// HasEnoughForAllowance checks whether an account has enough balance to cover for the allowance
func (s *StateReader) HasEnoughForAllowance(agentID isc.AgentID, allowance *isc.Assets) bool {
	if allowance == nil || allowance.IsEmpty() {
		return true
	}
	accountKey := s.accountKey(agentID)
	if allowance != nil {
		bts, _ := s.getBaseTokens(accountKey)
		if bts < allowance.BaseTokens {
			return false
		}
		for id, amount := range allowance.NativeTokens {
			if s.getNativeTokenAmount(accountKey, id).Cmp(amount) < 0 {
				return false
			}
		}
	}
	for _, nftID := range allowance.NFTs {
		if !s.hasNFT(agentID, nftID) {
			return false
		}
	}
	return true
}

// MoveBetweenAccounts moves assets between on-chain accounts
func (s *StateWriter) MoveBetweenAccounts(
	fromAgentID, toAgentID isc.AgentID,
	assets *isc.Assets,
) error {
	if fromAgentID.Equals(toAgentID) {
		// no need to move
		return nil
	}
	if assets == nil || assets.IsEmpty() {
		return nil
	}

	bts := util.BaseTokensDecimalsToEthereumDecimals(assets.FungibleTokens.BaseTokens, s.ctx.TokenInfo().Decimals)
	if !s.debitFromAccount(s.accountKey(fromAgentID), bts, assets.FungibleTokens.NativeTokens) {
		return errors.New("MoveBetweenAccounts: not enough funds")
	}
	s.creditToAccount(s.accountKey(toAgentID), bts, assets.FungibleTokens.NativeTokens)

	for _, nftID := range assets.NFTs {
		nft := s.GetNFTData(nftID)
		if nft == nil {
			return fmt.Errorf("MoveBetweenAccounts: unknown NFT %s", nftID)
		}
		if !s.debitNFTFromAccount(fromAgentID, nft) {
			return errors.New("MoveBetweenAccounts: NFT not found in origin account")
		}
		s.creditNFTToAccount(toAgentID, nft.ID, nft.Issuer)
	}

	s.touchAccount(fromAgentID)
	s.touchAccount(toAgentID)
	return nil
}

// debitBaseTokensFromAllowance is used for adjustment of L2 when part of base tokens are taken for storage deposit
// It takes base tokens from allowance to the common account and then removes them from the L2 ledger
func debitBaseTokensFromAllowance(ctx isc.Sandbox, amount iotago.BaseToken, chainID isc.ChainID) {
	if amount == 0 {
		return
	}
	storageDepositAssets := isc.NewFungibleTokens(amount, nil)
	ctx.TransferAllowedFunds(CommonAccount(), storageDepositAssets.ToAssets())
	NewStateWriterFromSandbox(ctx).DebitFromAccount(CommonAccount(), storageDepositAssets)
}

func (s *StateWriter) UpdateLatestOutputID(anchorTxID iotago.TransactionID, blockIndex uint32) {
	s.updateNativeTokenOutputIDs(anchorTxID)
	s.updateFoundryOutputIDs(anchorTxID)
	s.updateNFTOutputIDs(anchorTxID)
	s.updateNewlyMintedNFTOutputIDs(anchorTxID, blockIndex)
}
