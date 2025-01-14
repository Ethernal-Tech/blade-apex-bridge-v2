package cardanofw

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	infracommon "github.com/Ethernal-Tech/cardano-infrastructure/common"
	"github.com/Ethernal-Tech/cardano-infrastructure/sendtx"
	"github.com/Ethernal-Tech/cardano-infrastructure/wallet"
)

const (
	potentialFee      = 250_000
	ttlSlotNumberInc  = 500
	bridgingFeeAmount = uint64(1_100_000)
	maxInputsPerTx    = 16
)

func SendTx(ctx context.Context,
	txProvider wallet.ITxProvider,
	cardanoWallet *wallet.Wallet,
	amount uint64,
	receiver string,
	networkType wallet.CardanoNetworkType,
	metadata []byte,
) (txHash string, err error) {
	return infracommon.ExecuteWithRetry(ctx, func(ctx context.Context) (string, error) {
		return sendTx(ctx, txProvider, cardanoWallet, amount, receiver, networkType, metadata)
	})
}

func sendTx(ctx context.Context,
	txProvider wallet.ITxProvider,
	cardanoWallet *wallet.Wallet,
	amount uint64,
	receiver string,
	networkType wallet.CardanoNetworkType,
	metadata []byte,
) (string, error) {
	caddr, err := GetAddress(networkType, cardanoWallet)
	if err != nil {
		return "", err
	}

	cardanoWalletAddr := caddr.String()

	txSender := sendtx.NewTxSender(
		bridgingFeeAmount,
		MinUTxODefaultValue,
		potentialFee,
		maxInputsPerTx,
		map[string]sendtx.ChainConfig{
			GetNetworkName(networkType): {
				CardanoCliBinary:   ResolveCardanoCliBinary(networkType),
				TxProvider:         txProvider,
				MultiSigAddr:       cardanoWalletAddr,
				TestNetMagic:       GetNetworkMagic(networkType),
				TTLSlotNumberInc:   ttlSlotNumberInc,
				MinUtxoValue:       MinUTxODefaultValue,
				ExchangeRate:       make(map[string]float64),
				ProtocolParameters: nil,
			},
		},
	)

	rawTx, txHash, err := txSender.CreateTxGeneric(
		ctx,
		GetNetworkName(networkType),
		cardanoWalletAddr,
		receiver,
		metadata,
		amount,
		0,
	)
	if err != nil {
		fmt.Printf("Error creating tx: %v\n", err)

		return "", err
	}

	return txHash, txSender.SubmitTx(ctx, GetNetworkName(networkType), rawTx, cardanoWallet)
}

func GetGenesisWalletFromCluster(
	dirPath string,
	keyID uint,
) (*wallet.Wallet, error) {
	keyFileName := strings.Join([]string{"utxo", fmt.Sprint(keyID)}, "")

	sKey, err := wallet.NewKey(filepath.Join(dirPath, "utxo-keys", fmt.Sprintf("%s.skey", keyFileName)))
	if err != nil {
		return nil, err
	}

	sKeyBytes, err := sKey.GetKeyBytes()
	if err != nil {
		return nil, err
	}

	vKey, err := wallet.NewKey(filepath.Join(dirPath, "utxo-keys", fmt.Sprintf("%s.vkey", keyFileName)))
	if err != nil {
		return nil, err
	}

	vKeyBytes, err := vKey.GetKeyBytes()
	if err != nil {
		return nil, err
	}

	return wallet.NewWallet(vKeyBytes, sKeyBytes), nil
}
