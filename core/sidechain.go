/*
 * Copyright (C) 2021 The Zion Authors
 * This file is part of The Zion library.
 *
 * The Zion is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The Zion is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The Zion.  If not, see <http://www.gnu.org/licenses/>.
 */

package core

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/palettechain/deploy-tool/config"
	"github.com/palettechain/deploy-tool/pkg/log"
	"github.com/palettechain/deploy-tool/pkg/poly"
	"github.com/palettechain/deploy-tool/pkg/sdk"
	polyutils "github.com/polynetwork/poly/native/service/utils"
)

func PLTRegisterSideChain() (succeed bool) {
	polyRPC := config.Conf.PolyRPCUrl
	polyValidators := config.Conf.LoadPolyAccountList()
	polyCli, err := poly.NewPolyClient(polyRPC, polyValidators)
	if err != nil {
		log.Errorf("failed to generate poly client, err: %s", err)
		return
	} else {
		log.Infof("generate poly client success!")
	}

	crossChainID := config.Conf.PaletteSideChainID
	eccd := config.Conf.PaletteECCD
	router := polyutils.QUORUM_ROUTER
	name := config.Conf.PaletteSideChainName
	if err := polyCli.RegisterSideChain(crossChainID, eccd, router, name); err != nil {
		log.Errorf("failed to register side chain, err: %s", err)
		return
	}

	log.Infof("register side chain %d eccd %s success", crossChainID, eccd.Hex())
	return true
}

func PLTApproveRegisterSideChain() (succeed bool) {
	polyRPC := config.Conf.PolyRPCUrl
	polyValidators := config.Conf.LoadPolyAccountList()
	polyCli, err := poly.NewPolyClient(polyRPC, polyValidators)
	if err != nil {
		log.Errorf("failed to generate poly client, err: %s", err)
		return
	} else {
		log.Infof("generate poly client success!")
	}

	crossChainID := config.Conf.PaletteSideChainID
	if err := polyCli.ApproveRegisterSideChain(crossChainID); err != nil {
		log.Errorf("failed to approve register side chain, err: %s", err)
		return
	}

	log.Infof("approve register side chain %d success", crossChainID)
	return true
}

// ??????palette????????????poly??????
// 1. ???????????????palette cli: ????????????palette??????????????????cli, poly cli: ?????????poly???????????????validators??????????????????
// 2. ??????palette????????????????????????, ?????????json????????????bytes
// 3. ??????poly cli??????????????????bytes??????palette network id???poly native????????????,
//	  ???????????????????????????poly??????????????????????????????, ???????????????????????????
// 4. ??????poly????????????????????????palette???????????????genesis????????????????????????block??????block header???block book keeper
//    ?????????????????????palette????????????
func PLTSyncPLTGenesis() (succeed bool) {
	// 1. prepare
	polyRPC := config.Conf.PolyRPCUrl
	polyValidators := config.Conf.LoadPolyAccountList()
	polyCli, err := poly.NewPolyClient(polyRPC, polyValidators)
	if err != nil {
		log.Errorf("failed to generate poly client, err: %s", err)
		return
	} else {
		log.Infof("generate poly client success!")
	}

	// 2. get palette current block header
	logsplit()
	cli := sdk.NewSender(config.Conf.PaletteRPCUrl, nil)
	//cli, err := getPaletteCli()
	//if err != nil {
	//	log.Errorf("get palette cross chain admin client failed")
	//	return
	//}
	curr, hdr, err := cli.GetCurrentBlockHeader()
	if err != nil {
		log.Errorf("failed to get block header, err: %s", err)
		return
	}
	pltHeaderEnc, err := hdr.MarshalJSON()
	if err != nil {
		log.Errorf("marshal header failed, err: %s", err)
		return
	}
	log.Infof("get palette block header with current height %d, header %s", curr, hexutil.Encode(pltHeaderEnc))

	logsplit()
	crossChainID := config.Conf.PaletteSideChainID
	if err := polyCli.SyncGenesisBlock(crossChainID, pltHeaderEnc); err != nil {
		log.Errorf("SyncEthGenesisHeader failed: %v", err)
		return
	}
	log.Infof("sync palette genesis header to poly success, txhash %s, block number %d",
		hdr.Hash().Hex(), hdr.Number.Uint64())

	return true
}

// ??????poly????????????palette
func PLTSyncPolyGenesis() (succeed bool) {
	polyRPC := config.Conf.PolyRPCUrl
	polyCli, err := poly.NewPolyClient(polyRPC, nil)
	if err != nil {
		log.Errorf("failed to generate poly client, err: %s", err)
		return
	} else {
		log.Infof("generate poly client success!")
	}

	// `epoch` related with the poly validators changing,
	// we can set it as 0 if poly validators never changed on develop environment.
	var hasValidatorsBlockNumber uint32 = 0
	gB, err := polyCli.GetBlockByHeight(hasValidatorsBlockNumber)
	if err != nil {
		log.Errorf("failed to get block, err: %s", err)
		return
	}
	bookeepers, err := poly.GetBookeeper(gB)
	if err != nil {
		log.Errorf("failed to get bookeepers, err: %s", err)
		return
	}
	bookeepersEnc := poly.AssembleNoCompressBookeeper(bookeepers)
	headerEnc := gB.Header.ToArray()

	cli, err := getPaletteCli()
	if err != nil {
		log.Errorf("get palette cross chain admin client failed")
		return
	}
	eccm := config.Conf.PaletteECCM
	txhash, err := cli.InitGenesisBlock(eccm, headerEnc, bookeepersEnc)
	if err != nil {
		log.Errorf("failed to initGenesisBlock, err: %s", err)
		return
	}

	log.Infof("sync poly genesis header to palette success, txhash %s, block number %d",
		txhash.Hex(), gB.Header.Height)

	return true
}
