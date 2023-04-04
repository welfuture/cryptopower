package exchange

import (
	"context"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"code.cryptopower.dev/group/cryptopower/app"
	"code.cryptopower.dev/group/cryptopower/libwallet/instantswap"
	"code.cryptopower.dev/group/cryptopower/listeners"
	"code.cryptopower.dev/group/cryptopower/ui/cryptomaterial"
	"code.cryptopower.dev/group/cryptopower/ui/load"
	"code.cryptopower.dev/group/cryptopower/ui/page/components"
	"code.cryptopower.dev/group/cryptopower/ui/values"
	"code.cryptopower.dev/group/cryptopower/wallet"

	api "code.cryptopower.dev/group/instantswap"
)

const OrderHistoryPageID = "OrderHistory"

type OrderHistoryPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	*listeners.OrderNotificationListener

	ctx       context.Context // page context
	ctxCancel context.CancelFunc

	listContainer *widget.List

	orderItems []*instantswap.Order
	ordersList *cryptomaterial.ClickableList

	materialLoader material.LoaderStyle

	backButton cryptomaterial.IconButton

	refreshClickable *cryptomaterial.Clickable
	refreshIcon      *cryptomaterial.Image
	statusDropdown   *cryptomaterial.DropDown

	loading, initialLoadingDone, loadedAll bool
}

func NewOrderHistoryPage(l *load.Load) *OrderHistoryPage {
	pg := &OrderHistoryPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(OrderHistoryPageID),
		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
		refreshClickable: l.Theme.NewClickable(true),
		refreshIcon:      l.Theme.Icons.Restore,
	}

	pg.backButton, _ = components.SubpageHeaderButtons(l)

	pg.materialLoader = material.Loader(l.Theme.Base)

	pg.ordersList = pg.Theme.NewClickableList(layout.Vertical)
	pg.ordersList.IsShadowEnabled = true

	pg.statusDropdown = l.Theme.DropDown([]cryptomaterial.DropDownItem{
		{Text: api.OrderStatusWaitingForDeposit.String()},
		{Text: api.OrderStatusDepositReceived.String()},
		{Text: api.OrderStatusNew.String()},
		{Text: api.OrderStatusCompleted.String()},
		{Text: api.OrderStatusExpired.String()},
	}, values.OrderStatusDropdownGroup, 0)

	return pg
}

func (pg *OrderHistoryPage) ID() string {
	return OrderHistoryPageID
}

func (pg *OrderHistoryPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())

	pg.listenForSyncNotifications()
	go pg.fetchOrders(false)
}

func (pg *OrderHistoryPage) OnNavigatedFrom() {
	if pg.ctxCancel != nil {
		pg.ctxCancel()
	}
}

func (pg *OrderHistoryPage) HandleUserInteractions() {
	for pg.statusDropdown.Changed() {
		pg.fetchOrders(false)
	}

	if clicked, selectedItem := pg.ordersList.ItemClicked(); clicked {
		selectedOrder := pg.orderItems[selectedItem]
		pg.ParentNavigator().Display(NewOrderDetailsPage(pg.Load, selectedOrder))
	}

	if pg.refreshClickable.Clicked() {
		go pg.WL.AssetsManager.InstantSwap.Sync(pg.ctx)
	}
}

func (pg *OrderHistoryPage) Layout(gtx C) D {

	pg.onScrollChangeListener()

	container := func(gtx C) D {
		sp := components.SubPage{
			Load:       pg.Load,
			Title:      values.String(values.StrOrderHistory),
			BackButton: pg.backButton,
			Back: func() {
				pg.ParentNavigator().CloseCurrentPage()
			},
			Body: func(gtx C) D {
				return layout.Stack{}.Layout(gtx, layout.Expanded(pg.layout))
			},
		}

		return cryptomaterial.LinearLayout{
			Width:     cryptomaterial.MatchParent,
			Height:    cryptomaterial.MatchParent,
			Direction: layout.Center,
		}.Layout2(gtx, func(gtx C) D {
			return cryptomaterial.LinearLayout{
				Width:     gtx.Dp(values.MarginPadding550),
				Height:    cryptomaterial.MatchParent,
				Alignment: layout.Middle,
			}.Layout2(gtx, func(gtx C) D {
				return sp.Layout(pg.ParentWindow(), gtx)

			})
		})
	}

	return components.UniformPadding(gtx, container)
}

func (pg *OrderHistoryPage) layout(gtx C) D {
	return cryptomaterial.LinearLayout{
		Width:     cryptomaterial.MatchParent,
		Height:    cryptomaterial.MatchParent,
		Direction: layout.Center,
	}.Layout2(gtx, func(gtx C) D {
		return cryptomaterial.LinearLayout{
			Width:  gtx.Dp(values.MarginPadding550),
			Height: cryptomaterial.MatchParent,
			Margin: layout.Inset{
				Bottom: values.MarginPadding30,
			},
		}.Layout2(gtx, func(gtx C) D {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx C) D {
					return layout.Inset{}.Layout(gtx, func(gtx C) D {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx C) D {
								return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
									layout.Flexed(1, func(gtx C) D {
										body := func(gtx C) D {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													var text string
													if pg.WL.AssetsManager.InstantSwap.IsSyncing() {
														text = values.String(values.StrSyncingState)
													} else {
														text = values.String(values.StrUpdated) + " " + components.TimeAgo(pg.WL.AssetsManager.InstantSwap.GetLastSyncedTimeStamp())

														if pg.WL.AssetsManager.InstantSwap.GetLastSyncedTimeStamp() == 0 {
															text = values.String(values.StrNeverSynced)
														}
													}

													lastUpdatedInfo := pg.Theme.Label(values.TextSize12, text)
													lastUpdatedInfo.Color = pg.Theme.Color.GrayText2
													return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, lastUpdatedInfo.Layout)
												}),
												layout.Rigid(func(gtx C) D {
													return cryptomaterial.LinearLayout{
														Width:     cryptomaterial.WrapContent,
														Height:    cryptomaterial.WrapContent,
														Clickable: pg.refreshClickable,
														Direction: layout.Center,
														Alignment: layout.Middle,
														Margin:    layout.Inset{Left: values.MarginPadding10},
													}.Layout(gtx,
														layout.Rigid(func(gtx C) D {
															if pg.WL.AssetsManager.InstantSwap.IsSyncing() {
																gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding8)
																gtx.Constraints.Min.X = gtx.Constraints.Max.X
																return layout.Inset{Bottom: values.MarginPadding1}.Layout(gtx, pg.materialLoader.Layout)
															}
															return layout.Inset{Right: values.MarginPadding16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
																return pg.refreshIcon.LayoutSize(gtx, values.MarginPadding18)
															})

														}),
													)
												}),
											)
										}
										return layout.E.Layout(gtx, body)
									}),
								)
							}),
							layout.Flexed(1, func(gtx C) D {
								return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
									return layout.Stack{}.Layout(gtx,
										layout.Expanded(func(gtx C) D {
											return layout.Inset{
												Top: values.MarginPadding60,
											}.Layout(gtx, pg.layoutHistory)
										}),
										layout.Expanded(func(gtx C) D {
											return pg.statusDropdown.Layout(gtx, 10, true)
										}),
									)
								})
							}),
						)
					})
				}),
			)
		})
	})
}

func (pg *OrderHistoryPage) fetchOrders(loadMore bool) {
	selectedStatus := pg.statusDropdown.Selected()
	var statusFilter api.Status
	switch selectedStatus {
	case api.OrderStatusWaitingForDeposit.String():
		statusFilter = api.OrderStatusWaitingForDeposit
	case api.OrderStatusDepositReceived.String():
		statusFilter = api.OrderStatusDepositReceived
	case api.OrderStatusNew.String():
		statusFilter = api.OrderStatusNew
	case api.OrderStatusRefunded.String():
		statusFilter = api.OrderStatusRefunded
	case api.OrderStatusExpired.String():
		statusFilter = api.OrderStatusExpired
	case api.OrderStatusCompleted.String():
		statusFilter = api.OrderStatusCompleted
	default:
		statusFilter = api.OrderStatusUnknown
	}

	if pg.loading {
		return
	}
	defer func() {
		pg.loading = false
	}()
	pg.loadedAll = false
	pg.loading = true

	limit := 10

	offset := 0
	if loadMore {
		offset = len(pg.orderItems)
	}

	tempOrders := components.LoadOrders(pg.Load, int32(offset), int32(limit), true, statusFilter)
	if tempOrders == nil {
		pg.orderItems = nil
		return
	}

	pg.initialLoadingDone = true

	if len(tempOrders) == 0 {
		pg.loadedAll = true
		pg.loading = false

		if !loadMore {
			pg.orderItems = nil
		}
		return
	}

	if len(tempOrders) < limit {
		pg.loadedAll = true
	}

	if loadMore {
		pg.orderItems = append(pg.orderItems, tempOrders...)
	} else {
		pg.orderItems = tempOrders
	}

	pg.ParentWindow().Reload()
}

func (pg *OrderHistoryPage) layoutHistory(gtx C) D {
	if len(pg.orderItems) == 0 {
		return components.LayoutNoOrderHistory(gtx, pg.Load, false)
	}

	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			return pg.Theme.List(pg.listContainer).Layout(gtx, 1, func(gtx C, i int) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return pg.ordersList.Layout(gtx, len(pg.orderItems), func(gtx C, i int) D {
						return cryptomaterial.LinearLayout{
							Orientation: layout.Vertical,
							Width:       cryptomaterial.MatchParent,
							Height:      cryptomaterial.WrapContent,
							Background:  pg.Theme.Color.Surface,
							Direction:   layout.W,
							Border:      cryptomaterial.Border{Radius: cryptomaterial.Radius(14)},
							Padding:     layout.UniformInset(values.MarginPadding15),
							Margin:      layout.Inset{Bottom: values.MarginPadding4, Top: values.MarginPadding4},
						}.Layout2(gtx, func(gtx C) D {
							return components.OrderItemWidget(gtx, pg.Load, pg.orderItems[i])
						})
					})
				})
			})
		}),
	)
}

func (pg *OrderHistoryPage) listenForSyncNotifications() {
	if pg.OrderNotificationListener != nil {
		return
	}
	pg.OrderNotificationListener = listeners.NewOrderNotificationListener()
	err := pg.WL.AssetsManager.InstantSwap.AddNotificationListener(pg.OrderNotificationListener, OrderHistoryPageID)
	if err != nil {
		log.Errorf("Error adding instanswap notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-pg.OrderNotifChan:
				if n.OrderStatus == wallet.OrderStatusSynced {
					pg.fetchOrders(false)
					pg.ParentWindow().Reload()
				}
			case <-pg.ctx.Done():
				pg.WL.AssetsManager.InstantSwap.RemoveNotificationListener(OrderHistoryPageID)
				close(pg.OrderNotifChan)
				pg.OrderNotificationListener = nil

				return
			}
		}
	}()
}

func (pg *OrderHistoryPage) onScrollChangeListener() {
	if len(pg.orderItems) < 5 || !pg.initialLoadingDone {
		return
	}

	// The first check is for when the list is scrolled to the bottom using the scroll bar.
	// The second check is for when the list is scrolled to the bottom using the mouse wheel.
	// OffsetLast is 0 if we've scrolled to the last item on the list. Position.Length > 0
	// is to check if the page is still scrollable.
	if (pg.listContainer.List.Position.OffsetLast >= -50 && pg.listContainer.List.Position.BeforeEnd) || (pg.listContainer.List.Position.OffsetLast == 0 && pg.listContainer.List.Position.Length > 0) {
		if !pg.loadedAll {
			pg.fetchOrders(true)
		}
	}
}
