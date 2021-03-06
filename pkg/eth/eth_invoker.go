/*
* Copyright (C) 2020 The poly network Authors
* This file is part of The poly network library.
*
* The poly network is free software: you can redistribute it and/or modify
* it under the terms of the GNU Lesser General Public License as published by
* the Free Software Foundation, either version 3 of the License, or
* (at your option) any later version.
*
* The poly network is distributed in the hope that it will be useful,
* but WITHOUT ANY WARRANTY; without even the implied warranty of
* MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
* GNU Lesser General Public License for more details.
* You should have received a copy of the GNU Lesser General Public License
* along with The poly network . If not, see <http://www.gnu.org/licenses/>.
 */
package eth

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contracts/native/utils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/palettechain/deploy-tool/pkg/log"
	// pltabi "github.com/palettechain/palette_token/go_abi/plt"
	"github.com/polynetwork/eth-contracts/go_abi/eccd_abi"
	"github.com/polynetwork/eth-contracts/go_abi/eccm_abi"
	"github.com/polynetwork/eth-contracts/go_abi/eccmp_abi"
	"github.com/polynetwork/eth-contracts/go_abi/lock_proxy_abi"
	nftlp "github.com/polynetwork/nft-contracts/go_abi/nft_lock_proxy_abi"
	nftmapping "github.com/polynetwork/nft-contracts/go_abi/nft_mapping_abi"
)

// 部署在以太上的PLT token来自项目github.com/palettechain/palette-token.git
// 该项目的proxy和admin合约用于合约升级，并不是跨链使用的lockProxy。
// 而对应的proxy和NFT的proxy一样，来自项目github.com/polynetwork/eth-contracts.git

type EthInvoker struct {
	PrivateKey *ecdsa.PrivateKey
	Tools      *ETHTools
	NM         *NonceManager
	TestSigner *EthSigner
}

var (
	DefaultGasLimit = 100000
)

func NewEInvoker(url string, privateKey *ecdsa.PrivateKey) *EthInvoker {
	instance := &EthInvoker{}
	instance.Tools = NewEthTools(url)
	if instance.Tools == nil {
		log.Errorf("dail eth failed")
	}
	instance.NM = NewNonceManager(instance.Tools.GetEthClient())
	instance.PrivateKey = privateKey
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	instance.TestSigner = &EthSigner{
		PrivateKey: privateKey,
		Address:    address,
	}
	return instance
}

func (i *EthInvoker) Address() common.Address {
	publicKey := i.PrivateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		panic("ecdsa public key convert failed")
	}
	return crypto.PubkeyToAddress(*publicKeyECDSA)
}

func (i *EthInvoker) DeployPLTLockProxy() (common.Address, error) {
	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyAddress, err
	}
	contractAddr, tx, _, err := lock_proxy_abi.DeployLockProxy(auth, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyAddress, err
	}
	return contractAddr, nil
}

func (i *EthInvoker) DeployNFTLockProxy() (common.Address, error) {
	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyAddress, err
	}
	contractAddr, tx, _, err := nftlp.DeployPolyNFTLockProxy(auth, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyAddress, err
	}
	return contractAddr, nil
}

func (i *EthInvoker) SetPLTCCMP(proxyAddr, ccmpAddr common.Address) (common.Hash, error) {
	proxy, err := lock_proxy_abi.NewLockProxy(proxyAddr, i.backend())
	if err != nil {
		return utils.EmptyHash, err
	}
	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := proxy.SetManagerProxy(auth, ccmpAddr)
	if err != nil {
		return utils.EmptyHash, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) GetPLTCCMP(proxyAddr common.Address) (common.Address, error) {
	proxy, err := lock_proxy_abi.NewLockProxy(proxyAddr, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}

	return proxy.ManagerProxyContract(nil)
}

func (i *EthInvoker) SetNFTCCMP(proxyAddr, ccmpAddr common.Address) (common.Hash, error) {
	proxy, err := nftlp.NewPolyNFTLockProxy(proxyAddr, i.backend())
	if err != nil {
		return utils.EmptyHash, err
	}
	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := proxy.SetManagerProxy(auth, ccmpAddr)
	if err != nil {
		return utils.EmptyHash, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) GetNFTCCMP(proxyAddr common.Address) (common.Address, error) {
	proxy, err := nftlp.NewPolyNFTLockProxy(proxyAddr, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}

	return proxy.ManagerProxyContract(nil)
}

func (i *EthInvoker) DeployNFT(name, symbol string) (common.Address, error) {
	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyAddress, err
	}
	address, tx, inst, err := nftmapping.DeployCrossChainNFTMapping(auth, i.backend(), name, symbol)
	if err != nil {
		return utils.EmptyAddress, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyAddress, err
	}
	nameAfterDeploy, err := inst.Name(nil)
	if err != nil {
		return utils.EmptyAddress, err
	}
	if nameAfterDeploy != name {
		return utils.EmptyAddress, fmt.Errorf("mapping contract deployed name %s != %s", nameAfterDeploy, name)
	}
	return address, nil
}

func (i *EthInvoker) BindPLTAsset(
	localLockProxyAddr,
	fromAssetHash,
	toAssetHash common.Address,
	toChainId uint64,
) (common.Hash, error) {

	proxy, err := lock_proxy_abi.NewLockProxy(localLockProxyAddr, i.backend())
	if err != nil {
		return utils.EmptyHash, err
	}

	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := proxy.BindAssetHash(auth, fromAssetHash, toChainId, toAssetHash[:])
	if err != nil {
		return utils.EmptyHash, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) GetBoundPLTAsset(
	localLockProxyAddr,
	fromAssetHash common.Address,
	toChainId uint64,
) (common.Address, error) {

	proxy, err := lock_proxy_abi.NewLockProxy(localLockProxyAddr, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}

	bz, err := proxy.AssetHashMap(nil, fromAssetHash, toChainId)
	if err != nil {
		return utils.EmptyAddress, err
	}

	return common.BytesToAddress(bz), nil
}

func (i *EthInvoker) BindPLTProxy(
	localLockProxy,
	targetLockProxy common.Address,
	targetSideChainID uint64,
) (common.Hash, error) {

	proxy, err := lock_proxy_abi.NewLockProxy(localLockProxy, i.backend())
	if err != nil {
		return utils.EmptyHash, err
	}

	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := proxy.BindProxyHash(auth, targetSideChainID, targetLockProxy.Bytes())
	if err != nil {
		return utils.EmptyHash, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) GetBoundPLTProxy(
	localLockProxy common.Address,
	targetSideChainID uint64,
) (common.Address, error) {

	proxy, err := lock_proxy_abi.NewLockProxy(localLockProxy, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}

	bz, err := proxy.ProxyHashMap(nil, targetSideChainID)
	if err != nil {
		return utils.EmptyAddress, err
	}

	return common.BytesToAddress(bz), nil
}

func (i *EthInvoker) BindNFTAsset(
	lockProxyAddr,
	fromAssetHash,
	toAssetHash common.Address,
	targetSideChainId uint64) (common.Hash, error) {

	proxy, err := nftlp.NewPolyNFTLockProxy(lockProxyAddr, i.backend())
	if err != nil {
		return utils.EmptyHash, err
	}

	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := proxy.BindAssetHash(auth, fromAssetHash, targetSideChainId, toAssetHash[:])
	if err != nil {
		return utils.EmptyHash, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) GetBoundNFTAsset(
	lockProxyAddr,
	fromAssetHash common.Address,
	targetSideChainId uint64,
) (common.Address, error) {

	proxy, err := nftlp.NewPolyNFTLockProxy(lockProxyAddr, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}

	bz, err := proxy.AssetHashMap(nil, fromAssetHash, targetSideChainId)
	if err != nil {
		return utils.EmptyAddress, err
	}

	return common.BytesToAddress(bz), nil
}

func (i *EthInvoker) BindNFTProxy(
	localLockProxy,
	targetLockProxy common.Address,
	targetSideChainID uint64,
) (common.Hash, error) {
	proxy, err := nftlp.NewPolyNFTLockProxy(localLockProxy, i.backend())
	if err != nil {
		return utils.EmptyHash, err
	}
	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := proxy.BindProxyHash(auth, targetSideChainID, targetLockProxy.Bytes())
	if err != nil {
		return utils.EmptyHash, err
	}

	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) GetBoundNFTProxy(
	localLockProxy common.Address,
	targetSideChainID uint64,
) (common.Address, error) {
	proxy, err := nftlp.NewPolyNFTLockProxy(localLockProxy, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}

	bz, err := proxy.ProxyHashMap(nil, targetSideChainID)
	if err != nil {
		return utils.EmptyAddress, err
	}

	return common.BytesToAddress(bz), nil
}

func (i *EthInvoker) TransferECCDOwnership(eccd, eccm common.Address) (common.Hash, error) {
	eccdContract, err := eccd_abi.NewEthCrossChainData(eccd, i.backend())
	if err != nil {
		return utils.EmptyHash, fmt.Errorf("TransferECCDOwnership, err: %v", err)
	}
	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := eccdContract.TransferOwnership(auth, eccm)
	if err != nil {
		return utils.EmptyHash, fmt.Errorf("TransferECCDOwnership, err: %v", err)
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) ECCDOwnership(eccdAddr common.Address) (common.Address, error) {
	eccd, err := eccd_abi.NewEthCrossChainData(eccdAddr, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}
	return eccd.Owner(nil)
}

func (i *EthInvoker) TransferECCMOwnership(eccm, ccmp common.Address) (common.Hash, error) {
	eccmContract, err := eccm_abi.NewEthCrossChainManager(eccm, i.backend())
	if err != nil {
		return utils.EmptyHash, fmt.Errorf("TransferECCMOwnership err: %v", err)
	}
	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := eccmContract.TransferOwnership(auth, ccmp)
	if err != nil {
		return utils.EmptyHash, fmt.Errorf("TransferECCMOwnership err: %v", err)
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) ECCMOwnership(eccmAddr common.Address) (common.Address, error) {
	eccm, err := eccm_abi.NewEthCrossChainManager(eccmAddr, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}
	return eccm.Owner(nil)
}

func (i *EthInvoker) TransferCCMPOwnership(ccmpAddr, newOwner common.Address) (common.Hash, error) {
	ccmp, err := eccmp_abi.NewEthCrossChainManagerProxy(ccmpAddr, i.backend())
	if err != nil {
		return utils.EmptyHash, err
	}

	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := ccmp.TransferOwnership(auth, newOwner)
	if err != nil {
		return utils.EmptyHash, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) CCMPOwnership(ccmpAddr common.Address) (common.Address, error) {
	ccmp, err := eccmp_abi.NewEthCrossChainManagerProxy(ccmpAddr, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}
	return ccmp.Owner(nil)
}

func (i *EthInvoker) TransferPLTAssetOwnership(asset, newOwner common.Address) (common.Hash, error) {
	//instance, err := pltabi.NewPaletteToken(asset, i.backend())
	//if err != nil {
	//	return utils.EmptyHash, err
	//}
	//
	//auth, err := i.makeAuth()
	//if err != nil {
	//	return utils.EmptyHash, err
	//}
	//tx, err := instance.TransferOwnership(auth, newOwner)
	//if err != nil {
	//	return utils.EmptyHash, err
	//}
	//if err := i.waitTxConfirm(tx.Hash()); err != nil {
	//	return utils.EmptyHash, err
	//}
	//return tx.Hash(), nil
	return utils.EmptyHash, nil
}

//func (i *EthInvoker) AcceptOwnership(asset common.Address) (common.Hash, error) {
//	instance, err := pltabi.NewPaletteToken(asset, i.backend())
//	if err != nil {
//		return utils.EmptyHash, err
//	}
//
//	auth, err := i.makeAuth()
//	if err != nil {
//		return utils.EmptyHash, err
//	}
//	tx, err := instance.AcceptOwnership(auth)
//	if err != nil {
//		return utils.EmptyHash, err
//	}
//	if err := i.waitTxConfirm(tx.Hash()); err != nil {
//		return utils.EmptyHash, err
//	}
//	return tx.Hash(), nil
//}
//
//func (i *EthInvoker) PLTAssetOwnership(asset common.Address) (common.Address, error) {
//	instance, err := pltabi.NewPaletteToken(asset, i.backend())
//	if err != nil {
//		return utils.EmptyAddress, err
//	}
//	return instance.Owner(nil)
//}

func (i *EthInvoker) TransferPLTProxyOwnership(proxyAddr, newOwner common.Address) (common.Hash, error) {
	proxy, err := lock_proxy_abi.NewLockProxy(proxyAddr, i.backend())
	if err != nil {
		return utils.EmptyHash, err
	}

	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := proxy.TransferOwnership(auth, newOwner)
	if err != nil {
		return utils.EmptyHash, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) PLTProxyOwnership(proxyAddr common.Address) (common.Address, error) {
	proxy, err := lock_proxy_abi.NewLockProxy(proxyAddr, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}
	return proxy.Owner(nil)
}

func (i *EthInvoker) TransferNFTProxyOwnership(proxyAddr, newOwner common.Address) (common.Hash, error) {
	proxy, err := nftlp.NewPolyNFTLockProxy(proxyAddr, i.backend())
	if err != nil {
		return utils.EmptyHash, err
	}

	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := proxy.TransferOwnership(auth, newOwner)
	if err != nil {
		return utils.EmptyHash, err
	}
	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) NFTProxyOwnership(proxyAddr common.Address) (common.Address, error) {
	proxy, err := nftlp.NewPolyNFTLockProxy(proxyAddr, i.backend())
	if err != nil {
		return utils.EmptyAddress, err
	}
	return proxy.Owner(nil)
}

func (i *EthInvoker) DumpTx(hash common.Hash) error {
	tx, err := i.GetReceipt(hash)
	if err != nil {
		return fmt.Errorf("faild to get receipt %s", hash.Hex())
	}

	if tx.Status == 0 {
		return fmt.Errorf("receipt failed %s", hash.Hex())
	}

	log.Infof("txhash %s, block height %d", hash.Hex(), tx.BlockNumber.Uint64())
	for _, event := range tx.Logs {
		log.Infof("eventlog address %s", event.Address.Hex())
		log.Infof("eventlog data %s", new(big.Int).SetBytes(event.Data).String())
		for i, topic := range event.Topics {
			log.Infof("eventlog topic[%d] %s", i, topic.String())
		}
	}
	return nil
}

func (i *EthInvoker) GetReceipt(hash common.Hash) (*types.Receipt, error) {
	tx, err := i.Tools.ethclient.TransactionReceipt(context.Background(), hash)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (i *EthInvoker) GetCurrentHeight() (uint64, error) {
	return i.Tools.GetNodeHeight()
}

func (i *EthInvoker) GetHeader(height uint64) (*types.Header, error) {
	return i.Tools.GetBlockHeader(height)
}

func (i *EthInvoker) InitGenesisBlock(eccmAddr common.Address, rawHdr, publickeys []byte) (common.Hash, error) {
	eccm, err := eccm_abi.NewEthCrossChainManager(eccmAddr, i.backend())
	if err != nil {
		return utils.EmptyHash, fmt.Errorf("new EthCrossChainManager err: %s", err)
	}

	auth, err := i.makeAuth()
	if err != nil {
		return utils.EmptyHash, err
	}
	tx, err := eccm.InitGenesisBlock(auth, rawHdr, publickeys)
	if err != nil {
		return utils.EmptyHash, fmt.Errorf("call eccm InitGenesisBlock err: %s", err)
	}

	if err := i.waitTxConfirm(tx.Hash()); err != nil {
		return utils.EmptyHash, err
	}
	return tx.Hash(), nil
}

func (i *EthInvoker) SuggestGasPrice() (*big.Int, error) {
	return i.backend().SuggestGasPrice(context.Background())
}

func (i *EthInvoker) makeAuth() (*bind.TransactOpts, error) {
	fromAddress := i.Address()
	nonce, err := i.backend().PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return nil, fmt.Errorf("makeAuth, addr %s, err %v", fromAddress.Hex(), err)
	}

	gasPrice, err := i.backend().SuggestGasPrice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("makeAuth, %v", err)
	}

	auth := bind.NewKeyedTransactor(i.PrivateKey)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(int64(0))       // in wei
	auth.GasLimit = uint64(DefaultGasLimit) // in units
	auth.GasPrice = gasPrice.Mul(gasPrice, big.NewInt(1))

	return auth, nil
}

func (i *EthInvoker) waitTxConfirm(hash common.Hash) error {
	i.Tools.WaitTransactionConfirm(hash)
	if err := i.DumpTx(hash); err != nil {
		return err
	}
	return nil
}

func (i *EthInvoker) backend() bind.ContractBackend {
	return i.Tools.GetEthClient()
}
