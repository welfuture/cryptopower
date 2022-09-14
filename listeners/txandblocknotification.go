package listeners

import (
	"encoding/json"

	"gitlab.com/raedah/cryptopower/libwallet"
)

// TxAndBlockNotificationListener satisfies libwallet
// TxAndBlockNotificationListener interface contract.
type TxAndBlockNotificationListener struct {
	TxAndBlockNotifChan chan TxNotification
}

func NewTxAndBlockNotificationListener() *TxAndBlockNotificationListener {
	return &TxAndBlockNotificationListener{
		TxAndBlockNotifChan: make(chan TxNotification, 4),
	}
}

func (txAndBlk *TxAndBlockNotificationListener) OnTransaction(transaction string) {
	var tx libwallet.Transaction
	err := json.Unmarshal([]byte(transaction), &tx)
	if err != nil {
		log.Errorf("Error unmarshalling transaction: %v", err)
		return
	}

	update := TxNotification{
		Type:        NewTransaction,
		Transaction: &tx,
	}
	txAndBlk.UpdateNotification(update)
}

func (txAndBlk *TxAndBlockNotificationListener) OnBlockAttached(walletID int, blockHeight int32) {
	txAndBlk.UpdateNotification(TxNotification{
		Type:        BlockAttached,
		WalletID:    walletID,
		BlockHeight: blockHeight,
	})
}

func (txAndBlk *TxAndBlockNotificationListener) OnTransactionConfirmed(walletID int, hash string, blockHeight int32) {
	txAndBlk.UpdateNotification(TxNotification{
		Type:        TxConfirmed,
		WalletID:    walletID,
		BlockHeight: blockHeight,
		Hash:        hash,
	})
}

func (txAndBlk *TxAndBlockNotificationListener) UpdateNotification(signal TxNotification) {
	txAndBlk.TxAndBlockNotifChan <- signal
}
