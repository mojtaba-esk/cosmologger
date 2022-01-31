package tx

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/archway-network/cosmologger/configs"
	"github.com/archway-network/cosmologger/database"
	tmClient "github.com/tendermint/tendermint/rpc/client/http"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmTypes "github.com/tendermint/tendermint/types"
)

func ProcessEvents(db *database.Database, evr coretypes.ResultEvent) error {

	// jsonString, err := json.MarshalIndent(evr, "", "  ")
	// if err != nil {
	// return err
	// }

	rec := GetTxRecordFromEvent(evr)

	// tx := evr.Data.(tmTypes.EventDataTx)
	fmt.Printf("\n\n\t=====================================================\n\nevr.Events: \n")
	for i := range evr.Events {
		fmt.Printf("\nType: %v", i)
		for j := range evr.Events[i] {
			fmt.Printf("\tAttr: %v ==> %s\n", j, evr.Events[i][j])
		}
	}

	dbRow := rec.GetDBRow()
	delete(dbRow, database.FIELD_TX_EVENTS_TX_MEMO)
	delete(dbRow, database.FIELD_TX_EVENTS_LOG_TIME) // This will be set by the DB itself
	db.Insert(database.TABLE_TX_EVENTS, dbRow)

	return nil
}

func GetTxRecordFromEvent(evr coretypes.ResultEvent) TxRecord {
	var txRecord TxRecord

	if evr.Events["tx.height"] != nil && len(evr.Events["tx.height"]) > 0 {
		txRecord.Height, _ = strconv.ParseUint(evr.Events["tx.height"][0], 10, 64)
	}

	if evr.Events["tx.hash"] != nil && len(evr.Events["tx.hash"]) > 0 {
		txRecord.TxHash = evr.Events["tx.hash"][0]
	}

	if evr.Events["message.module"] != nil && len(evr.Events["message.module"]) > 0 {
		txRecord.Module = evr.Events["message.module"][0]
	}

	if evr.Events["message.sender"] != nil && len(evr.Events["message.sender"]) > 0 {
		txRecord.Sender = evr.Events["message.sender"][0]

	} else if evr.Events["transfer.sender"] != nil && len(evr.Events["transfer.sender"]) > 0 {

		txRecord.Sender = evr.Events["transfer.sender"][0]
	}

	if evr.Events["transfer.recipient"] != nil && len(evr.Events["transfer.recipient"]) > 0 {
		txRecord.Receiver = evr.Events["transfer.recipient"][0]
	}

	if evr.Events["delegate.validator"] != nil && len(evr.Events["delegate.validator"]) > 0 {
		txRecord.Validator = evr.Events["delegate.validator"][0]

	} else if evr.Events["create_validator.validator"] != nil && len(evr.Events["create_validator.validator"]) > 0 {

		txRecord.Validator = evr.Events["create_validator.validator"][0]
	}

	if evr.Events["message.action"] != nil && len(evr.Events["message.action"]) > 0 {
		txRecord.Action = evr.Events["message.action"][0]
	}

	if evr.Events["delegate.amount"] != nil && len(evr.Events["delegate.amount"]) > 0 {
		txRecord.Amount = evr.Events["delegate.amount"][0]

	} else if evr.Events["transfer.amount"] != nil && len(evr.Events["transfer.amount"]) > 0 {

		txRecord.Amount = evr.Events["transfer.amount"][0]
	}

	if evr.Events["tx.acc_seq"] != nil && len(evr.Events["tx.acc_seq"]) > 0 {
		txRecord.TxAccSeq = evr.Events["tx.acc_seq"][0]
	}

	if evr.Events["tx.signature"] != nil && len(evr.Events["tx.signature"]) > 0 {
		txRecord.TxSignature = evr.Events["tx.signature"][0]
	}

	if evr.Events["proposal_vote.proposal_id"] != nil && len(evr.Events["proposal_vote.proposal_id"]) > 0 {
		txRecord.ProposalId, _ = strconv.ParseUint(evr.Events["proposal_vote.proposal_id"][0], 10, 64)

	} else if evr.Events["proposal_deposit.proposal_id"] != nil && len(evr.Events["proposal_deposit.proposal_id"]) > 0 {

		txRecord.ProposalId, _ = strconv.ParseUint(evr.Events["proposal_deposit.proposal_id"][0], 10, 64)
	}

	// Memo cannot be retrieved through tx events, we may fill it up with another way later
	// txRecord.TxMemo =

	jsonBytes, err := json.Marshal(evr.Events)
	if err == nil {
		txRecord.Json = string(jsonBytes)
	}

	// LogTime: is recorded by the DBMS itself

	return txRecord
}

func (t *TxRecord) GetDBRow() database.RowType {
	return database.RowType{

		database.FIELD_TX_EVENTS_TX_HASH:      t.TxHash,
		database.FIELD_TX_EVENTS_HEIGHT:       t.Height,
		database.FIELD_TX_EVENTS_MODULE:       t.Module,
		database.FIELD_TX_EVENTS_SENDER:       t.Sender,
		database.FIELD_TX_EVENTS_RECEIVER:     t.Receiver,
		database.FIELD_TX_EVENTS_VALIDATOR:    t.Validator,
		database.FIELD_TX_EVENTS_ACTION:       t.Action,
		database.FIELD_TX_EVENTS_AMOUNT:       t.Amount,
		database.FIELD_TX_EVENTS_TX_ACCSEQ:    t.TxAccSeq,
		database.FIELD_TX_EVENTS_TX_SIGNATURE: t.TxSignature,
		database.FIELD_TX_EVENTS_PROPOSAL_ID:  t.ProposalId,
		database.FIELD_TX_EVENTS_TX_MEMO:      t.TxMemo,
		database.FIELD_TX_EVENTS_JSON:         t.Json,
		database.FIELD_TX_EVENTS_LOG_TIME:     t.LogTime,
	}
}

func Start(cli *tmClient.HTTP, db *database.Database) {

	go func() {

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(configs.Configs.GRPC.CallTimeout))
		defer cancel()

		// eventChan, err := cli.Subscribe(ctx, "cosmologger", tmTypes.QueryForEvent(tmTypes.EventNewBlock).String())
		eventChan, err := cli.Subscribe(ctx, "cosmologger", tmTypes.QueryForEvent(tmTypes.EventTx).String())
		if err != nil {
			panic(err)
		}

		for {
			fmt.Println("\nWaiting for the new signal...")
			evRes := <-eventChan
			ProcessEvents(db, evRes)
		}
	}()
}