package sdk

import (
	"context"
	"github.com/ethereum/go-ethereum/contracts/native/plt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/native/utils"
	"github.com/palettechain/deploy-tool/pkg/log"
)

func (c *Client) BalanceOf(owner common.Address, blockNum string) (*big.Int, error) {
	payload, err := c.packPLT(plt.MethodBalanceOf, owner)
	if err != nil {
		return nil, err
	}

	enc, err := c.callPLT(payload, blockNum)
	if err != nil {
		return nil, err
	}

	output := new(plt.MethodBalanceOfOutput)
	if err := c.unpackPLT(plt.MethodBalanceOf, output, enc); err != nil {
		return nil, err
	}

	return output.Balance, nil
}

func (self *Client) WaitTransaction(hash common.Hash) error {
	for {
		time.Sleep(time.Second * 1)
		_, ispending, err := self.backend.TransactionByHash(context.Background(), hash)
		if err != nil {
			log.Errorf("failed to call TransactionByHash: %v", err)
			continue
		}
		if ispending == true {
			continue
		}

		if err := self.DumpEventLog(hash); err != nil {
			return err
		}
		break
	}
	return nil
}

func (c *Client) packPLT(method string, args ...interface{}) ([]byte, error) {
	return utils.PackMethod(PLTABI, method, args...)
}
func (c *Client) unpackPLT(method string, output interface{}, enc []byte) error {
	return utils.UnpackOutputs(PLTABI, method, output, enc)
}
func (c *Client) sendPLTTx(payload []byte) (common.Hash, error) {
	hash, err := c.SendTransaction(PLTAddress, payload)
	if err != nil {
		return utils.EmptyHash, err
	}
	if err := c.WaitTransaction(hash); err != nil {
		return utils.EmptyHash, err
	}
	return hash, nil
}
func (c *Client) callPLT(payload []byte, blockNum string) ([]byte, error) {
	return c.CallContract(c.Address(), PLTAddress, payload, blockNum)
}
