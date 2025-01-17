package e2e

import (
	"testing"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/e2e-polybft/framework"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/Ethernal-Tech/cardano-infrastructure/indexer"
	cardanowallet "github.com/Ethernal-Tech/cardano-infrastructure/wallet"
	"github.com/Ethernal-Tech/ethgo/abi"
	"github.com/stretchr/testify/require"
)

func TestE2E_ApexBridge_TestCardanoVerifySignaturePrecompile(t *testing.T) {
	admin, err := wallet.GenerateAccount()
	require.NoError(t, err)

	cluster := framework.NewTestCluster(t, 4,
		framework.WithBladeAdmin(admin.Address().String()),
	)

	defer cluster.Stop()

	cluster.WaitForReady(t)

	txRelayer, err := txrelayer.NewTxRelayer(txrelayer.WithClient(cluster.Servers[0].JSONRPC()))
	require.NoError(t, err)

	// deploy contract
	receipt, err := txRelayer.SendTransaction(
		types.NewTx(types.NewLegacyTx(
			types.WithFrom(admin.Ecdsa.Address()),
			types.WithInput(contractsapi.TestCardanoVerifySign.Bytecode),
		)),
		admin.Ecdsa)
	require.NoError(t, err)

	dummyKeyHash := []byte("0837676734ffaab34")
	contractAddr := types.Address(receipt.ContractAddress)
	txRaw, _ := hex.DecodeString("84a50083825820098236134e0f2077a6434dd9d7727126fa8b3627bcab3ae030a194d46eded73e00825820d1fd0d772be7741d9bfaf0b037d02d2867a987ccba3e6ba2ee9aa2a861b7314502825820e99a5bde15aa05f24fcc04b7eabc1520d3397283b1ee720de9fe2653abbb0c9f00018382581d60244877c1aeefc7fd5405a6e14d927d91758d45e37c20fa2ac89cb1671a000f424082581d700c25e4ff24cfa0dfebcec382095161271dc9bb744ca4149ec604dc991a001e847682581d70a5caf9ce4bed09c794ee87bddb6505822db5bd476a4f61e0cd4074a21a001a79bf021a00040ac103196dc0075820802e4d6f15ce98826886a5451e94855e77aae779cb341d3aab1e3bae4fb2f78da10182830304858200581cd6b67f93ffa4e2651271cc9bcdbdedb2539911266b534d9c163cba218200581ccba89c7084bf0ce4bf404346b668a7e83c8c9c250d1cafd8d8996e418200581c79df3577e4c7d7da04872c2182b8d8829d7b477912dbf35d89287c398200581c2368e8113bd5f32d713751791d29acee9e1b5a425b0454b963b2558b8200581c06b4c7f5254d6395b527ac3de60c1d77194df7431d85fe55ca8f107d830304858200581cf0f4837b3a306752a2b3e52394168bc7391de3dce11364b723cc55cf8200581c47344d5bd7b2fea56336ba789579705a944760032585ef64084c92db8200581cf01018c1d8da54c2f557679243b09af1c4dd4d9c671512b01fa5f92b8200581c6837232854849427dae7c45892032d7ded136c5beb13c68fda635d878200581cd215701e2eb17c741b9d306cba553f9fbaaca1e12a5925a065b90fa8f5d90103a100a200a36a6665655369676e65727305677369676e657273056474797065656d756c746904a26463697479684e6f76692053616464636f6d706845746865726e616c")

	txInfo, err := indexer.ParseTxInfo(txRaw)
	require.NoError(t, err)

	txHash, _ := hex.DecodeString(txInfo.Hash)
	invalidKey := [32]byte{}

	cw, err := cardanowallet.GenerateWallet(false)
	require.NoError(t, err)

	checkValidity := func(t *testing.T,
		txRawOrMsg []byte, signature []byte, verificationKey []byte, isTx bool, shouldBeValid bool) {
		t.Helper()

		var fn *abi.Method
		if isTx {
			fn = contractsapi.TestCardanoVerifySign.Abi.GetMethod("check")
		} else {
			fn = contractsapi.TestCardanoVerifySign.Abi.GetMethod("checkMsg")
		}

		hasQuorumFnBytes, err := fn.Encode([]interface{}{
			txRawOrMsg, signature, [32]byte(verificationKey),
		})
		require.NoError(t, err)

		response, err := txRelayer.Call(types.ZeroAddress, contractAddr, hasQuorumFnBytes)
		require.NoError(t, err)

		if shouldBeValid {
			require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000001", response)
		} else {
			require.Equal(t, "0x0000000000000000000000000000000000000000000000000000000000000000", response)
		}
	}

	signature, err := cardanowallet.SignMessage(cw.SigningKey, cw.VerificationKey, txHash)
	require.NoError(t, err)

	checkValidity(t, txRaw, signature, cw.VerificationKey, true, true)
	checkValidity(t, txRaw, signature, invalidKey[:], true, false)
	checkValidity(t, txRaw, append([]byte{0}, signature...), cw.VerificationKey, true, false)

	// message with key hash example
	signature, err = cardanowallet.SignMessage(
		cw.SigningKey, cw.VerificationKey, append([]byte("hello world: "), dummyKeyHash...))
	require.NoError(t, err)

	checkValidity(t, dummyKeyHash, signature, cw.VerificationKey, false, true)
	checkValidity(t, dummyKeyHash, signature, invalidKey[:], false, false)
	checkValidity(t, dummyKeyHash, append([]byte{0}, signature...), cw.VerificationKey, false, false)
}
