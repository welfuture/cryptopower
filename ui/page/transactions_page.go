package page

import (
	"context"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/planetdecred/dcrlibwallet"
	"github.com/planetdecred/godcr/ui/decredmaterial"
	"github.com/planetdecred/godcr/ui/load"
	"github.com/planetdecred/godcr/ui/page/components"
	"github.com/planetdecred/godcr/ui/values"
	"github.com/planetdecred/godcr/wallet"
)

const TransactionsPageID = "Transactions"

type transactionWdg struct {
	statusIcon           *widget.Image
	direction            *widget.Image
	time, status, wallet decredmaterial.Label
}

type TransactionsPage struct {
	*load.Load
	ctx       context.Context // page context
	ctxCancel context.CancelFunc
	container layout.Flex
	separator decredmaterial.Line

	orderDropDown   *decredmaterial.DropDown
	txTypeDropDown  *decredmaterial.DropDown
	walletDropDown  *decredmaterial.DropDown
	transactionList *decredmaterial.ClickableList

	transactions []dcrlibwallet.Transaction
	wallets      []*dcrlibwallet.Wallet
}

func NewTransactionsPage(l *load.Load) *TransactionsPage {
	pg := &TransactionsPage{
		Load:            l,
		container:       layout.Flex{Axis: layout.Vertical},
		separator:       l.Theme.Separator(),
		transactionList: l.Theme.NewClickableList(layout.Vertical),
	}

	pg.orderDropDown = components.CreateOrderDropDown(l)
	pg.txTypeDropDown = l.Theme.DropDown([]decredmaterial.DropDownItem{
		{
			Text: values.String(values.StrAll),
		},
		{
			Text: values.String(values.StrSent),
		},
		{
			Text: values.String(values.StrReceived),
		},
		{
			Text: values.String(values.StrYourself),
		},
		{
			Text: values.String(values.StrStaking),
		},
	}, 1)

	return pg
}

func (pg *TransactionsPage) ID() string {
	return TransactionsPageID
}

func (pg *TransactionsPage) OnResume() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	pg.wallets = pg.WL.SortedWalletList()
	components.CreateOrUpdateWalletDropDown(pg.Load, &pg.walletDropDown, pg.wallets)
	pg.listenForTxNotifications()
	pg.loadTransactions()
}

func (pg *TransactionsPage) loadTransactions() {
	selectedWallet := pg.wallets[pg.walletDropDown.SelectedIndex()]
	newestFirst := pg.orderDropDown.SelectedIndex() == 0

	txFilter := dcrlibwallet.TxFilterAll
	switch pg.txTypeDropDown.SelectedIndex() {
	case 1:
		txFilter = dcrlibwallet.TxFilterSent
	case 2:
		txFilter = dcrlibwallet.TxFilterReceived
	case 3:
		txFilter = dcrlibwallet.TxFilterTransferred
	case 4:
		txFilter = dcrlibwallet.TxFilterStaking
	}

	wallTxs, err := selectedWallet.GetTransactionsRaw(0, 0, txFilter, newestFirst) //TODO
	if err != nil {
		log.Error("Error loading transactions:", err)
	} else {
		pg.transactions = wallTxs
	}
}

func (pg *TransactionsPage) Layout(gtx layout.Context) layout.Dimensions {
	container := func(gtx C) D {
		wallTxs := pg.transactions
		return layout.Stack{Alignment: layout.N}.Layout(gtx,
			layout.Expanded(func(gtx C) D {
				return layout.Inset{
					Top: values.MarginPadding60,
				}.Layout(gtx, func(gtx C) D {
					return pg.Theme.Card().Layout(gtx, func(gtx C) D {
						padding := values.MarginPadding16
						return components.Container{Padding: layout.Inset{Bottom: padding, Left: padding}}.Layout(gtx,
							func(gtx C) D {
								// return "No transactions yet" text if there are no transactions
								if len(wallTxs) == 0 {
									gtx.Constraints.Min.X = gtx.Constraints.Max.X
									txt := pg.Theme.Body1(values.String(values.StrNoTransactionsYet))
									txt.Color = pg.Theme.Color.Gray2
									return layout.Center.Layout(gtx, txt.Layout)
								}

								return pg.transactionList.Layout(gtx, len(wallTxs), func(gtx C, index int) D {
									var row = components.TransactionRow{
										Transaction: wallTxs[index],
										Index:       index,
										ShowBadge:   false,
									}
									return components.LayoutTransactionRow(gtx, pg.Load, row)
								})
							})
					})
				})
			}),
			layout.Stacked(pg.dropDowns),
		)
	}
	return components.UniformPadding(gtx, container)
}

func (pg *TransactionsPage) dropDowns(gtx layout.Context) layout.Dimensions {
	return layout.Inset{
		Bottom: values.MarginPadding10,
	}.Layout(gtx, func(gtx C) D {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Horizontal, Spacing: layout.SpaceBetween}.Layout(gtx,
			layout.Rigid(pg.walletDropDown.Layout),
			layout.Rigid(func(gtx C) D {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Left: values.MarginPadding5,
						}.Layout(gtx, pg.txTypeDropDown.Layout)
					}),
					layout.Rigid(func(gtx C) D {
						return layout.Inset{
							Left: values.MarginPadding5,
						}.Layout(gtx, pg.orderDropDown.Layout)
					}),
				)
			}),
		)
	})
}

func (pg *TransactionsPage) Handle() {
	for pg.txTypeDropDown.Changed() {
		pg.loadTransactions()
	}

	for pg.orderDropDown.Changed() {
		pg.loadTransactions()
	}

	for pg.walletDropDown.Changed() {
		pg.loadTransactions()
	}

	if clicked, selectedItem := pg.transactionList.ItemClicked(); clicked {
		pg.ChangeFragment(NewTransactionDetailsPage(pg.Load, &pg.transactions[selectedItem]))
	}
}

func (pg *TransactionsPage) listenForTxNotifications() {
	go func() {
		for {
			var notification interface{}

			select {
			case notification = <-pg.Receiver.NotificationsUpdate:
			case <-pg.ctx.Done():
				return
			}

			switch n := notification.(type) {
			case wallet.NewTransaction:
				selectedWallet := pg.wallets[pg.walletDropDown.SelectedIndex()]
				if selectedWallet.ID == n.Transaction.WalletID {
					pg.loadTransactions()
					pg.RefreshWindow()
				}
			}
		}
	}()
}

func (pg *TransactionsPage) OnClose() {
	pg.ctxCancel()
}

func initTxnWidgets(l *load.Load, transaction *dcrlibwallet.Transaction) transactionWdg {

	var txn transactionWdg
	t := time.Unix(transaction.Timestamp, 0).UTC()
	txn.time = l.Theme.Body1(t.Format(time.UnixDate))
	txn.status = l.Theme.Body1("")
	txn.wallet = l.Theme.Body2(l.WL.MultiWallet.WalletWithID(transaction.WalletID).Name)

	if components.TxConfirmations(l, *transaction) > 1 {
		txn.status.Text = components.FormatDateOrTime(transaction.Timestamp)
		txn.statusIcon = l.Icons.ConfirmIcon
	} else {
		txn.status.Text = "pending"
		txn.status.Color = l.Theme.Color.Gray
		txn.statusIcon = l.Icons.PendingIcon
	}

	if transaction.Direction == dcrlibwallet.TxDirectionSent {
		txn.direction = l.Icons.SendIcon
	} else {
		txn.direction = l.Icons.ReceiveIcon
	}

	return txn
}
