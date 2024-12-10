package e2ehelper

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/e2e-polybft/cardanofw"
	infracommon "github.com/Ethernal-Tech/cardano-infrastructure/common"
	"github.com/stretchr/testify/require"
)

func ExecuteSingleBridging(
	t *testing.T, ctx context.Context, apex IApexSystem, senderUser, receiverUser *cardanofw.TestApexUser,
	srcChain, dstChain string, sendAmountDfm *big.Int,
) {
	t.Helper()

	prevAmountDfm, err := apex.GetBalance(ctx, receiverUser, dstChain)
	require.NoError(t, err)

	txHash := apex.SubmitBridgingRequest(
		t, ctx, srcChain, dstChain, senderUser, sendAmountDfm, receiverUser)
	expectedAmountDfm := new(big.Int).Add(prevAmountDfm, sendAmountDfm)

	fmt.Printf("Tx sent. hash: %s\n", txHash)

	// check expected amount cardano
	err = apex.WaitForExactAmount(ctx, receiverUser, dstChain, expectedAmountDfm, 100, time.Second*10)
	require.NoError(t, err)
}

func ExecuteBridgingOneByOneWaitOnOtherSide(
	t *testing.T, ctx context.Context, apex IApexSystem, instances int,
	user *cardanofw.TestApexUser, srcChain, dstChain string, sendAmountDfm *big.Int,
) {
	t.Helper()

	for i := 0; i < instances; i++ {
		prevAmountDfm, err := apex.GetBalance(ctx, user, dstChain)
		require.NoError(t, err)

		apex.SubmitBridgingRequest(t, ctx, srcChain, dstChain, user, sendAmountDfm, user)
		expectedAmountDfm := new(big.Int).Add(prevAmountDfm, sendAmountDfm)

		err = apex.WaitForExactAmount(ctx, user, dstChain, expectedAmountDfm, 100, time.Second*10)
		require.NoError(t, err)
	}
}

func ExecuteBridgingWaitAfterSubmits(
	t *testing.T, ctx context.Context, apex IApexSystem, instances int,
	user *cardanofw.TestApexUser, srcChain, dstChain string, sendAmountDfm *big.Int,
) {
	t.Helper()

	prevAmountDfm, err := apex.GetBalance(ctx, user, dstChain)
	require.NoError(t, err)

	expectedAmountDfm := new(big.Int).Set(prevAmountDfm)

	for i := 0; i < instances; i++ {
		apex.SubmitBridgingRequest(t, ctx, srcChain, dstChain, user, sendAmountDfm, user)
		expectedAmountDfm = expectedAmountDfm.Add(expectedAmountDfm, sendAmountDfm)
	}

	err = apex.WaitForExactAmount(ctx, user, dstChain, expectedAmountDfm, 100, time.Second*10)
	require.NoError(t, err)
}

func ExecuteBridging(
	t *testing.T, ctx context.Context, apex IApexSystem, instances int,
	senderUsers []*cardanofw.TestApexUser, receiverUsers []*cardanofw.TestApexUser,
	chains []string, chainsDst map[string][]string,
	sendAmountDfm *big.Int, options ...ExecuteBridgingOption,
) {
	t.Helper()

	config := newExecuteBridgingConfig(options...)
	dstChains := getAllDestionationChains(chains, chainsDst)
	chainPairs := getAllChainPairs(chains, chainsDst)
	expectedAmountPerChainDfm := make([]map[string]*big.Int, len(receiverUsers))

	for i, receiverUser := range receiverUsers {
		expectedAmountPerChainDfm[i] = make(map[string]*big.Int)

		for _, dstChain := range dstChains {
			dfm, err := apex.GetBalance(ctx, receiverUser, dstChain)
			require.NoError(t, err)

			expectedAmountPerChainDfm[i][dstChain] = dfm
		}
	}

	var wg sync.WaitGroup

	for i, sender := range senderUsers {
		for _, chainPair := range chainPairs {
			wg.Add(1)

			go func(idx int, senderUser *cardanofw.TestApexUser, chainPair srcDstChainPair) {
				defer wg.Done()

				for j := 0; j < instances; j++ {
					txHash := apex.SubmitBridgingRequest(
						t, ctx, chainPair.srcChain, chainPair.dstChain, senderUser, sendAmountDfm, receiverUsers...)

					fmt.Printf("Sender: %d. run: %d. %s->%s tx sent: %s\n",
						idx+1, j+1, chainPair.srcChain, chainPair.dstChain, txHash)
				}
			}(i, sender, chainPair)
		}
	}

	wg.Wait()

	// update expectedAmountPerChainDfm
	for recieverUserIdx := range receiverUsers {
		for _, chainPair := range chainPairs {
			tmp := expectedAmountPerChainDfm[recieverUserIdx][chainPair.dstChain]
			tmp.Add(tmp, new(big.Int).Mul(sendAmountDfm, big.NewInt(int64(instances)*int64(len(senderUsers)))))
		}
	}

	if len(config.stopValidatorIndexes) > 0 {
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(config.stopValidatorAfter):
				for _, idx := range config.stopValidatorIndexes {
					require.NoError(t, apex.GetValidator(t, idx).Stop())
				}
			}
		}()
	}

	if len(config.startValidatorIndexes) > 0 {
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(config.startValidatorAfter):
				for _, idx := range config.startValidatorIndexes {
					require.NoError(t, apex.GetValidator(t, idx).Start(ctx, false))
				}
			}
		}()
	}

	var (
		wgResults sync.WaitGroup
		errs      = make([]error, len(receiverUsers)*len(dstChains))
	)

	for i, user := range receiverUsers {
		for j, dstChain := range dstChains {
			wgResults.Add(1)

			go func(idx int, idxChain int, receiverUser *cardanofw.TestApexUser, dstChain string, expectedAmountDfm *big.Int) {
				defer wgResults.Done()

				err := apex.WaitForExactAmount(
					ctx, receiverUser, dstChain, expectedAmountDfm, 100, time.Second*10)
				if err != nil {
					errs[idx*len(dstChains)+idxChain] = fmt.Errorf("receiver %d on %s: %w", idx, dstChain, err)

					return
				}

				fmt.Printf("TXs on %s for user %d expected amount received\n", dstChain, idx)

				if config.waitForUnexpectedBridges {
					// nothing else should be bridged for 2 minutes
					err = apex.WaitForGreaterAmount(
						ctx, receiverUser, dstChain, expectedAmountDfm, 12, time.Second*10)
					if !errors.Is(err, infracommon.ErrRetryTimeout) {
						errs[idx*len(dstChains)+idxChain] = fmt.Errorf(
							"receiver %d on %s should not receive more tokens: %w", idx, dstChain, err)

						return
					}

					fmt.Printf("TXs on %s for user %d finished with success\n", dstChain, idx)
				}
			}(i, j, user, dstChain, expectedAmountPerChainDfm[i][dstChain])
		}
	}

	wgResults.Wait()

	require.NoError(t, errors.Join(errs...))
}