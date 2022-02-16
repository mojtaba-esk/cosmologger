package block

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/archway-network/cosmologger/configs"
	"github.com/archway-network/cosmologger/database"
	"github.com/archway-network/cosmologger/validators"
	sdk "github.com/cosmos/cosmos-sdk/types"
	tmClient "github.com/tendermint/tendermint/rpc/client/http"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmTypes "github.com/tendermint/tendermint/types"
	"google.golang.org/grpc"
)

var genesisValidatorsDone bool

func ProcessEvents(db *database.Database, grpcCnn *grpc.ClientConn, evr *coretypes.ResultEvent) error {

	rec := getBlockRecordFromEvent(evr)
	fmt.Printf("Block: %s\tH: %d\tTxs: %d\n", rec.BlockHash, rec.Height, rec.NumOfTxs)

	dbRow := rec.getBlockDBRow()
	db.InsertAsync(database.TABLE_BLOCKS, dbRow)
	// _, err := db.Insert(database.TABLE_BLOCKS, dbRow)
	// if err != nil {
	// 	return err
	// }

	for i := range rec.Signers {

		dbRow := rec.Signers[i].getBlockSignerDBRow()
		db.InsertAsync(database.TABLE_BLOCK_SIGNERS, dbRow)
		// _, err := db.Insert(database.TABLE_BLOCK_SIGNERS, dbRow)
		// if err != nil {
		// 	return err
		// }
	}

	// Let's add genesis validator's info
	if !genesisValidatorsDone && rec.Height > 1 {
		// In case we miss the block 2
		genesisValidatorsDone = true

		// Just to make things non-blocking
		go func() {

			valList, err := validators.QueryValidatorsList(grpcCnn)
			if err != nil {
				log.Printf("Err in `validators.QueryValidatorsList`: %v", err)
				// return err
			}

			for i := range valList {
				err := validators.AddNewValidator(db, grpcCnn, valList[i])
				if err != nil {
					log.Printf("Err in `AddNewValidator`: %v", err)
					// return err
				}
			}

		}()
	}

	return nil
}

func getBlockRecordFromEvent(evr *coretypes.ResultEvent) *BlockRecord {
	var br BlockRecord

	b := evr.Data.(tmTypes.EventDataNewBlock)
	br.BlockHash = b.Block.Hash().String()

	br.Height = uint64(b.Block.Height)
	br.NumOfTxs = uint64(len(b.Block.Txs))
	br.Time = b.Block.Time

	for i := range b.Block.LastCommit.Signatures {

		consAddr, err := sdk.ConsAddressFromHex(b.Block.LastCommit.Signatures[i].ValidatorAddress.String())
		if err != nil {
			continue // just ignore this signer as it might not be running and we face some strange error
		}

		br.Signers = append(br.Signers, BlockSignersRecord{
			BlockHeight: br.Height,
			ValConsAddr: consAddr.String(),
			Time:        b.Block.LastCommit.Signatures[i].Timestamp,
			Signature:   base64.StdEncoding.EncodeToString(b.Block.LastCommit.Signatures[i].Signature),
		})
	}

	return &br
}

func (b *BlockRecord) getBlockDBRow() database.RowType {
	return database.RowType{

		database.FIELD_BLOCKS_BLOCK_HASH: b.BlockHash,
		database.FIELD_BLOCKS_HEIGHT:     b.Height,
		database.FIELD_BLOCKS_NUM_OF_TXS: b.NumOfTxs,
		database.FIELD_BLOCKS_TIME:       b.Time,
	}
}

func (s *BlockSignersRecord) getBlockSignerDBRow() database.RowType {
	return database.RowType{

		database.FIELD_BLOCK_SIGNERS_BLOCK_HEIGHT:  s.BlockHeight,
		database.FIELD_BLOCK_SIGNERS_VAL_CONS_ADDR: s.ValConsAddr,
		database.FIELD_BLOCK_SIGNERS_TIME:          s.Time,
		database.FIELD_BLOCK_SIGNERS_SIGNATURE:     s.Signature,
	}
}

func Start(cli *tmClient.HTTP, grpcCnn *grpc.ClientConn, db *database.Database) {

	go func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(configs.Configs.GRPC.CallTimeout))
		defer cancel()

		eventChan, err := cli.Subscribe(ctx, configs.Configs.TendermintClient.SubscriberName, tmTypes.QueryForEvent(tmTypes.EventNewBlock).String())
		if err != nil {
			panic(err)
		}

		for {
			evRes := <-eventChan
			err := ProcessEvents(db, grpcCnn, &evRes)
			if err != nil {
				//TODO: We need some customizable log level
				log.Printf("Error in processing block event: %v", err)
			}
		}
	}()
}
