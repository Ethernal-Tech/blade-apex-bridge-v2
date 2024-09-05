package common

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/Ethernal-Tech/ethgo"
	"github.com/spf13/cobra"

	cmdHelper "github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
)

type TokenType int

const (
	ERC20 TokenType = iota
	ERC721
	ERC1155
)

const (
	SenderKeyFlag          = "sender-key"
	ReceiversFlag          = "receivers"
	AmountsFlag            = "amounts"
	TokenIDsFlag           = "token-ids"
	RootTokenFlag          = "root-token"
	RootPredicateFlag      = "root-predicate"
	ChildPredicateFlag     = "child-predicate"
	ChildTokenFlag         = "child-token"
	JSONRPCFlag            = "json-rpc"
	ChildChainMintableFlag = "child-chain-mintable"

	MinterKeyFlag     = "minter-key"
	MinterKeyFlagDesc = "minter key is the account which is able to mint tokens to sender account " +
		"(if provided tokens are minted prior to depositing)"
)

var (
	errInconsistentAmounts  = errors.New("receivers and amounts must be equal length")
	errInconsistentTokenIds = errors.New("receivers and token ids must be equal length")
)

type BridgeParams struct {
	SenderKey          string
	Receivers          []string
	TokenAddr          string
	PredicateAddr      string
	JSONRPCAddr        string
	ChildChainMintable bool
	TxTimeout          time.Duration
}

// RegisterCommonFlags registers common bridge flags to a given command
func (p *BridgeParams) RegisterCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&p.SenderKey,
		SenderKeyFlag,
		"",
		"hex encoded private key of the account which sends bridge transactions",
	)

	cmd.Flags().StringSliceVar(
		&p.Receivers,
		ReceiversFlag,
		nil,
		"receiving accounts addresses",
	)

	cmd.Flags().StringVar(
		&p.JSONRPCAddr,
		JSONRPCFlag,
		txrelayer.DefaultRPCAddress,
		"the JSON RPC endpoint",
	)

	cmd.Flags().BoolVar(
		&p.ChildChainMintable,
		ChildChainMintableFlag,
		false,
		"flag indicating whether tokens originate from child chain",
	)

	cmd.Flags().DurationVar(
		&p.TxTimeout,
		cmdHelper.TxTimeoutFlag,
		txrelayer.DefaultTimeoutTransactions,
		cmdHelper.TxTimeoutDesc,
	)

}

func (p *BridgeParams) Validate() error {
	if p == nil {
		return nil
	}

	_, err := cmdHelper.ParseJSONRPCAddress(p.JSONRPCAddr)
	if err != nil {
		return fmt.Errorf("failed to parse json rpc address. Error: %w", err)
	}

	return nil
}

type ERC20BridgeParams struct {
	*BridgeParams
	Amounts []string
}

func NewERC20BridgeParams() *ERC20BridgeParams {
	return &ERC20BridgeParams{BridgeParams: &BridgeParams{}}
}

func (bp *ERC20BridgeParams) Validate() error {
	if err := bp.BridgeParams.Validate(); err != nil {
		return err
	}

	if len(bp.Receivers) != len(bp.Amounts) {
		return errInconsistentAmounts
	}

	return nil
}

type ERC721BridgeParams struct {
	*BridgeParams
	TokenIDs []string
}

func NewERC721BridgeParams() *ERC721BridgeParams {
	return &ERC721BridgeParams{BridgeParams: &BridgeParams{}}
}

func (bp *ERC721BridgeParams) Validate() error {
	if err := bp.BridgeParams.Validate(); err != nil {
		return err
	}

	if len(bp.Receivers) != len(bp.TokenIDs) {
		return errInconsistentTokenIds
	}

	return nil
}

type ERC1155BridgeParams struct {
	*BridgeParams
	Amounts  []string
	TokenIDs []string
}

func NewERC1155BridgeParams() *ERC1155BridgeParams {
	return &ERC1155BridgeParams{BridgeParams: &BridgeParams{}}
}

func (bp *ERC1155BridgeParams) Validate() error {
	if err := bp.BridgeParams.Validate(); err != nil {
		return err
	}

	if len(bp.Receivers) != len(bp.Amounts) {
		return errInconsistentAmounts
	}

	if len(bp.Receivers) != len(bp.TokenIDs) {
		return errInconsistentTokenIds
	}

	return nil
}

// ExtractBridgeMessageIDs tries to extract all bridgeMsg events ids from provided receipt
func ExtractBridgeMessageIDs(receipt *ethgo.Receipt) ([]*big.Int, error) {
	bridgeMsgEventIDs := make([]*big.Int, 0, len(receipt.Logs))

	for _, log := range receipt.Logs {
		var bridgeMsgEvent contractsapi.BridgeMsgEvent

		doesMatch, err := bridgeMsgEvent.ParseLog(log)
		if err != nil {
			return nil, err
		}

		if !doesMatch {
			continue
		}

		bridgeMsgEventIDs = append(bridgeMsgEventIDs, bridgeMsgEvent.ID)
	}

	if len(bridgeMsgEventIDs) != 0 {
		return bridgeMsgEventIDs, nil
	}

	return nil, errors.New("failed to find bridgeMsg event log")
}

// ExtractChildTokenAddr extracts predicted deterministic child token address
func ExtractChildTokenAddr(receipt *ethgo.Receipt) (*types.Address, error) {
	var (
		tokenMapped contractsapi.TokenMappedEvent
	)

	for _, log := range receipt.Logs {
		doesMatch, err := tokenMapped.ParseLog(log)
		if err != nil {
			return nil, err
		}

		if !doesMatch {
			continue
		}

		return &tokenMapped.ChildToken, nil
	}

	return nil, nil
}

type BridgeTxResult struct {
	Sender            string         `json:"sender"`
	Receivers         []string       `json:"receivers"`
	BridgeMsgEventIDs []*big.Int     `json:"bridgeMsgEventIDs"`
	Amounts           []string       `json:"amounts"`
	TokenIDs          []string       `json:"tokenIds"`
	BlockNumbers      []uint64       `json:"blockNumbers"`
	ChildTokenAddr    *types.Address `json:"childTokenAddr"`

	Title string `json:"title"`
}

func (r *BridgeTxResult) GetOutput() string {
	var buffer bytes.Buffer

	vals := make([]string, 0, 7)
	vals = append(vals, fmt.Sprintf("Sender|%s", r.Sender))
	vals = append(vals, fmt.Sprintf("Receivers|%s", strings.Join(r.Receivers, ", ")))

	if len(r.Amounts) > 0 {
		vals = append(vals, fmt.Sprintf("Amounts|%s", strings.Join(r.Amounts, ", ")))
	}

	if len(r.TokenIDs) > 0 {
		vals = append(vals, fmt.Sprintf("Token Ids|%s", strings.Join(r.TokenIDs, ", ")))
	}

	if len(r.BridgeMsgEventIDs) > 0 {
		var buf bytes.Buffer

		for i, id := range r.BridgeMsgEventIDs {
			buf.WriteString(id.String())

			if i != len(r.BridgeMsgEventIDs)-1 {
				buf.WriteString(", ")
			}
		}

		vals = append(vals, fmt.Sprintf("Exit Event IDs|%s", buf.String()))
	}

	if len(r.BlockNumbers) > 0 {
		var buf bytes.Buffer

		for i, blockNum := range r.BlockNumbers {
			buf.WriteString(fmt.Sprintf("%d", blockNum))

			if i != len(r.BlockNumbers)-1 {
				buf.WriteString(", ")
			}
		}

		vals = append(vals, fmt.Sprintf("Inclusion Block Numbers|%s", buf.String()))
	}

	if r.ChildTokenAddr != nil {
		vals = append(vals, fmt.Sprintf("Child Token Address|%s", (*r.ChildTokenAddr).String()))
	}

	_, _ = buffer.WriteString(fmt.Sprintf("\n[%s]\n", r.Title))
	_, _ = buffer.WriteString(cmdHelper.FormatKV(vals))
	_, _ = buffer.WriteString("\n")

	return buffer.String()
}
