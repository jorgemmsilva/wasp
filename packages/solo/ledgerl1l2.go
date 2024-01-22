package solo

import (
	"math"
	"math/big"
	"sort"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	iotago "github.com/iotaledger/iota.go/v4"
	"github.com/iotaledger/wasp/packages/cryptolib"
	"github.com/iotaledger/wasp/packages/isc"
	"github.com/iotaledger/wasp/packages/kv/kvdecoder"
	"github.com/iotaledger/wasp/packages/util"
	"github.com/iotaledger/wasp/packages/vm/core/accounts"
)

// L2Accounts returns all accounts on the chain with non-zero balances
func (ch *Chain) L2Accounts() []isc.AgentID {
	d, err := ch.CallView(accounts.ViewAccounts.Message())
	require.NoError(ch.Env.T, err)
	return lo.Must(accounts.ViewAccounts.Output.DecodeAccounts(d, ch.ChainID))
}

func (ch *Chain) L2Ledger() map[string]*isc.Assets {
	accs := ch.L2Accounts()
	ret := make(map[string]*isc.Assets)
	for i := range accs {
		ret[string(accs[i].Bytes())] = ch.L2Assets(accs[i])
	}
	return ret
}

func (ch *Chain) L2LedgerString() string {
	l := ch.L2Ledger()
	keys := make([]string, 0, len(l))
	for aid := range l {
		keys = append(keys, aid)
	}
	sort.Strings(keys)
	ret := ""
	for _, aid := range keys {
		ret += aid + "\n"
		ret += "        " + l[aid].String() + "\n"
	}
	return ret
}

// L2Assets return all tokens contained in the on-chain account controlled by the 'agentID'
func (ch *Chain) L2Assets(agentID isc.AgentID) *isc.Assets {
	return ch.L2AssetsAtStateIndex(agentID, ch.LatestBlockIndex())
}

func (ch *Chain) L2AssetsAtStateIndex(agentID isc.AgentID, stateIndex uint32) *isc.Assets {
	chainState, err := ch.store.StateByIndex(stateIndex)
	require.NoError(ch.Env.T, err)
	assets := lo.Must(accounts.ViewBalance.Output.Decode(
		lo.Must(ch.CallViewAtState(chainState, accounts.ViewBalance.Message(&agentID))),
	)).ToAssets()
	assets.NFTs = ch.L2NFTs(agentID)
	return assets
}

func (ch *Chain) L2BaseTokens(agentID isc.AgentID) iotago.BaseToken {
	return ch.L2Assets(agentID).BaseTokens
}

func (ch *Chain) L2BaseTokensAtStateIndex(agentID isc.AgentID, stateIndex uint32) iotago.BaseToken {
	return ch.L2AssetsAtStateIndex(agentID, stateIndex).BaseTokens
}

func (ch *Chain) L2NFTs(agentID isc.AgentID) []iotago.NFTID {
	res, err := ch.CallView(accounts.ViewAccountNFTs.Message(&agentID))
	require.NoError(ch.Env.T, err)
	return lo.Must(accounts.ViewAccountNFTs.Output.Decode(res))
}

func (ch *Chain) L2NativeTokens(agentID isc.AgentID, nativeTokenID iotago.NativeTokenID) *big.Int {
	return ch.L2Assets(agentID).NativeTokens.ValueOrBigInt0(nativeTokenID)
}

func (ch *Chain) L2CommonAccountAssets() *isc.Assets {
	return ch.L2Assets(accounts.CommonAccount())
}

func (ch *Chain) L2CommonAccountBaseTokens() iotago.BaseToken {
	return ch.L2Assets(accounts.CommonAccount()).BaseTokens
}

func (ch *Chain) L2CommonAccountNativeTokens(nativeTokenID iotago.NativeTokenID) *big.Int {
	return ch.L2Assets(accounts.CommonAccount()).NativeTokens.ValueOrBigInt0(nativeTokenID)
}

// L2TotalAssets return total sum of ftokens contained in the on-chain accounts
func (ch *Chain) L2TotalAssets() *isc.FungibleTokens {
	r, err := ch.CallView(accounts.ViewTotalAssets.Message())
	require.NoError(ch.Env.T, err)
	return lo.Must(accounts.ViewTotalAssets.Output.Decode(r))
}

// L2TotalBaseTokens return total sum of base tokens in L2 (all accounts)
func (ch *Chain) L2TotalBaseTokens() iotago.BaseToken {
	return ch.L2TotalAssets().BaseTokens
}

func (ch *Chain) GetOnChainTokenIDs() []iotago.NativeTokenID {
	res, err := ch.CallView(accounts.ViewGetNativeTokenIDRegistry.Message())
	require.NoError(ch.Env.T, err)
	return lo.Must(accounts.ViewGetNativeTokenIDRegistry.Output.Decode(res))
}

func (ch *Chain) GetFoundryOutput(sn uint32) (*iotago.FoundryOutput, error) {
	res, err := ch.CallView(accounts.ViewFoundryOutput.Message(sn))
	if err != nil {
		return nil, err
	}
	out, err := accounts.ViewFoundryOutput.Output.Decode(res)
	if err != nil {
		return nil, err
	}
	return out.(*iotago.FoundryOutput), nil
}

func (ch *Chain) GetNativeTokenIDByFoundrySN(sn uint32) (iotago.NativeTokenID, error) {
	o, err := ch.GetFoundryOutput(sn)
	if err != nil {
		return iotago.NativeTokenID{}, err
	}
	return o.MustNativeTokenID(), nil
}

type foundryParams struct {
	ch   *Chain
	user *cryptolib.KeyPair
	sch  iotago.TokenScheme
}

const (
	SendToL2AccountGasBudgetBaseTokens     = 1 * isc.Million
	TransferAllowanceToGasBudgetBaseTokens = 1 * isc.Million
)

func (ch *Chain) NewFoundryParams(maxSupply *big.Int) *foundryParams { //nolint:revive
	ret := &foundryParams{
		ch: ch,
		sch: &iotago.SimpleTokenScheme{
			MaximumSupply: maxSupply,
			MeltedTokens:  big.NewInt(0),
			MintedTokens:  big.NewInt(0),
		},
	}
	return ret
}

func (fp *foundryParams) WithUser(user *cryptolib.KeyPair) *foundryParams {
	fp.user = user
	return fp
}

func (fp *foundryParams) WithTokenScheme(sch iotago.TokenScheme) *foundryParams {
	fp.sch = sch
	return fp
}

const (
	allowanceForFoundryStorageDeposit = 1 * isc.Million
	allowanceForModifySupply          = 1 * isc.Million
)

func (fp *foundryParams) CreateFoundry() (uint32, iotago.NativeTokenID, error) {
	var sch *iotago.TokenScheme
	if fp.sch != nil {
		sch = &fp.sch
	}
	user := fp.ch.OriginatorPrivateKey
	if fp.user != nil {
		user = fp.user
	}
	req := NewCallParams(accounts.FuncFoundryCreateNew.Message(sch)).
		WithAllowance(isc.NewAssetsBaseTokens(allowanceForFoundryStorageDeposit)).
		AddBaseTokens(allowanceForFoundryStorageDeposit).
		WithMaxAffordableGasBudget()

	_, estimate, err := fp.ch.EstimateGasOnLedger(req, user)
	if err != nil {
		return 0, iotago.NativeTokenID{}, err
	}
	req.WithGasBudget(estimate.GasBurned)
	res, err := fp.ch.PostRequestSync(req, user)
	if err != nil {
		return 0, iotago.NativeTokenID{}, err
	}
	resDeco := kvdecoder.New(res)
	retSN := resDeco.MustGetUint32(accounts.ParamFoundrySN)
	nativeTokenID, err := fp.ch.GetNativeTokenIDByFoundrySN(retSN)
	return retSN, nativeTokenID, err
}

func (ch *Chain) DestroyFoundry(sn uint32, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.FuncFoundryDestroy.Message(sn)).
		WithMaxAffordableGasBudget()
	_, err := ch.PostRequestSync(req, user)
	return err
}

func (ch *Chain) MintTokens(sn uint32, amount *big.Int, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.FuncFoundryModifySupply.MintTokens(sn, amount)).
		WithAllowance(isc.NewAssetsBaseTokens(allowanceForModifySupply)) // enough allowance is needed for the storage deposit when token is minted first on the chain
	_, rec, err := ch.EstimateGasOnLedger(req, user)
	if err != nil {
		return err
	}

	req.WithGasBudget(rec.GasBurned)
	if user == nil {
		user = ch.OriginatorPrivateKey
	}
	_, err = ch.PostRequestSync(req, user)
	return err
}

// DestroyTokensOnL2 destroys tokens (identified by foundry SN) on user's on-chain account
func (ch *Chain) DestroyTokensOnL2(nativeTokenID iotago.NativeTokenID, amount *big.Int, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.FuncFoundryModifySupply.DestroyTokens(nativeTokenID.FoundrySerialNumber(), amount)).
		WithAllowance(isc.NewAssets(0, iotago.NativeTokenSum{nativeTokenID: util.ToBigInt(amount)})).
		WithMaxAffordableGasBudget()

	if user == nil {
		user = ch.OriginatorPrivateKey
	}
	_, err := ch.PostRequestSync(req, user)
	return err
}

// DestroyTokensOnL1 sends tokens as ftokens and destroys in the same transaction
func (ch *Chain) DestroyTokensOnL1(nativeTokenID iotago.NativeTokenID, amount *big.Int, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.FuncFoundryModifySupply.DestroyTokens(nativeTokenID.FoundrySerialNumber(), amount)).
		WithMaxAffordableGasBudget().AddBaseTokens(1000)
	req.AddNativeTokens(nativeTokenID, amount)
	req.AddAllowanceNativeTokens(nativeTokenID, amount)
	if user == nil {
		user = ch.OriginatorPrivateKey
	}
	_, err := ch.PostRequestSync(req, user)
	return err
}

// DepositAssetsToL2 deposits ftokens on user's on-chain account, if user is nil, then chain owner is assigned
func (ch *Chain) DepositAssetsToL2(assets *isc.Assets, user *cryptolib.KeyPair) error {
	_, err := ch.PostRequestSync(
		NewCallParams(accounts.FuncDeposit.Message()).
			WithFungibleTokens(assets).
			WithGasBudget(math.MaxUint64),
		user,
	)
	return err
}

// TransferAllowanceTo sends an on-ledger request to transfer funds to target account (sends extra base tokens to the sender account to cover gas)
func (ch *Chain) TransferAllowanceTo(
	allowance *isc.Assets,
	targetAccount isc.AgentID,
	wallet *cryptolib.KeyPair,
	nft ...*isc.NFT,
) error {
	callParams := NewCallParams(accounts.FuncTransferAllowanceTo.Message(targetAccount)).
		WithAllowance(allowance).
		WithFungibleTokens(allowance.Clone().AddBaseTokens(TransferAllowanceToGasBudgetBaseTokens)).
		WithMaxAffordableGasBudget()

	if len(nft) > 0 {
		callParams.WithNFT(nft[0])
	}
	_, err := ch.PostRequestSync(callParams, wallet)
	return err
}

// DepositBaseTokensToL2 deposits ftokens on user's on-chain account
func (ch *Chain) DepositBaseTokensToL2(amount iotago.BaseToken, user *cryptolib.KeyPair) error {
	return ch.DepositAssetsToL2(isc.NewAssets(amount, nil), user)
}

func (ch *Chain) MustDepositBaseTokensToL2(amount iotago.BaseToken, user *cryptolib.KeyPair) {
	err := ch.DepositBaseTokensToL2(amount, user)
	require.NoError(ch.Env.T, err)
}

func (ch *Chain) DepositNFT(nft *isc.NFT, to isc.AgentID, owner *cryptolib.KeyPair) error {
	return ch.TransferAllowanceTo(
		isc.NewEmptyAssets().AddNFTs(nft.ID),
		to,
		owner,
		nft,
	)
}

func (ch *Chain) MustDepositNFT(nft *isc.NFT, to isc.AgentID, owner *cryptolib.KeyPair) {
	err := ch.DepositNFT(nft, to, owner)
	require.NoError(ch.Env.T, err)
}

// Withdraw sends assets from the L2 account to L1
func (ch *Chain) Withdraw(assets *isc.Assets, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.FuncWithdraw.Message()).
		AddAllowance(assets).
		WithGasBudget(math.MaxUint64)
	if assets.BaseTokens == 0 {
		req.AddAllowance(isc.NewAssetsBaseTokens(1 * isc.Million)) // for storage deposit
	}
	_, err := ch.PostRequestOffLedger(req, user)
	return err
}

// SendFromL1ToL2Account sends ftokens from L1 address to the target account on L2
// Sender pays the gas fee
func (ch *Chain) SendFromL1ToL2Account(totalBaseTokens iotago.BaseToken, toSend *isc.Assets, target isc.AgentID, user *cryptolib.KeyPair) error {
	require.False(ch.Env.T, toSend.IsEmpty())
	sumAssets := toSend.Clone().AddBaseTokens(totalBaseTokens)
	_, err := ch.PostRequestSync(
		NewCallParams(accounts.FuncTransferAllowanceTo.Message(target)).
			AddAssets(sumAssets).
			AddAllowance(toSend).
			WithGasBudget(math.MaxUint64),
		user,
	)
	return err
}

func (ch *Chain) SendFromL1ToL2AccountBaseTokens(totalBaseTokens, baseTokensSend iotago.BaseToken, target isc.AgentID, user *cryptolib.KeyPair) error {
	return ch.SendFromL1ToL2Account(totalBaseTokens, isc.NewAssetsBaseTokens(baseTokensSend), target, user)
}

// SendFromL2ToL2Account moves ftokens on L2 from user's account to the target
func (ch *Chain) SendFromL2ToL2Account(transfer *isc.Assets, target isc.AgentID, user *cryptolib.KeyPair) error {
	req := NewCallParams(accounts.FuncTransferAllowanceTo.Message(target)).
		AddBaseTokens(SendToL2AccountGasBudgetBaseTokens).
		AddAllowance(transfer).
		WithMaxAffordableGasBudget()
	_, err := ch.PostRequestSync(req, user)
	return err
}

func (ch *Chain) SendFromL2ToL2AccountBaseTokens(baseTokens iotago.BaseToken, target isc.AgentID, user *cryptolib.KeyPair) error {
	return ch.SendFromL2ToL2Account(isc.NewAssetsBaseTokens(baseTokens), target, user)
}

func (ch *Chain) SendFromL2ToL2AccountNativeTokens(id iotago.NativeTokenID, target isc.AgentID, amount *big.Int, user *cryptolib.KeyPair) error {
	transfer := isc.NewEmptyAssets()
	transfer.AddNativeTokens(id, amount)
	return ch.SendFromL2ToL2Account(transfer, target, user)
}
