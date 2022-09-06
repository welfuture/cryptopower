package governance

import (
	"context"
	"sync"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gitlab.com/raedah/libwallet"
	"gitlab.com/raedah/cryptopower/app"
	"gitlab.com/raedah/cryptopower/listeners"
	"gitlab.com/raedah/cryptopower/ui/decredmaterial"
	"gitlab.com/raedah/cryptopower/ui/load"
	"gitlab.com/raedah/cryptopower/ui/modal"
	"gitlab.com/raedah/cryptopower/ui/page/components"
	"gitlab.com/raedah/cryptopower/ui/values"
	"gitlab.com/raedah/cryptopower/wallet"
)

const ProposalsPageID = "Proposals"

type (
	C = layout.Context
	D = layout.Dimensions
)

type ProposalsPage struct {
	*load.Load
	// GenericPageModal defines methods such as ID() and OnAttachedToNavigator()
	// that helps this Page satisfy the app.Page interface. It also defines
	// helper methods for accessing the PageNavigator that displayed this page
	// and the root WindowNavigator.
	*app.GenericPageModal

	*listeners.ProposalNotificationListener
	ctx        context.Context // page context
	ctxCancel  context.CancelFunc
	proposalMu sync.Mutex

	multiWallet      *libwallet.MultiWallet
	listContainer    *widget.List
	orderDropDown    *decredmaterial.DropDown
	categoryDropDown *decredmaterial.DropDown
	proposalsList    *decredmaterial.ClickableList
	syncButton       *widget.Clickable
	searchEditor     decredmaterial.Editor

	infoButton decredmaterial.IconButton

	updatedIcon *decredmaterial.Icon

	proposalItems []*components.ProposalItem

	syncCompleted bool
	isSyncing     bool
}

func NewProposalsPage(l *load.Load) *ProposalsPage {
	pg := &ProposalsPage{
		Load:             l,
		GenericPageModal: app.NewGenericPageModal(ProposalsPageID),
		multiWallet:      l.WL.MultiWallet,
		listContainer: &widget.List{
			List: layout.List{Axis: layout.Vertical},
		},
	}
	pg.searchEditor = l.Theme.IconEditor(new(widget.Editor), values.String(values.StrSearch), l.Theme.Icons.SearchIcon, true)
	pg.searchEditor.Editor.SingleLine, pg.searchEditor.Editor.Submit, pg.searchEditor.Bordered = true, true, false

	pg.updatedIcon = decredmaterial.NewIcon(pg.Theme.Icons.NavigationCheck)
	pg.updatedIcon.Color = pg.Theme.Color.Success

	pg.syncButton = new(widget.Clickable)

	pg.proposalsList = pg.Theme.NewClickableList(layout.Vertical)
	pg.proposalsList.IsShadowEnabled = true

	_, pg.infoButton = components.SubpageHeaderButtons(l)
	pg.infoButton.Size = values.MarginPadding20

	// orderDropDown is the first dropdown when page is laid out. Its
	// position should be 0 for consistent backdrop.
	pg.orderDropDown = components.CreateOrderDropDown(l, values.ProposalDropdownGroup, 0)
	pg.categoryDropDown = l.Theme.DropDown([]decredmaterial.DropDownItem{
		{
			Text: values.String(values.StrUnderReview),
		},
		{
			Text: values.String(values.StrApproved),
		},
		{
			Text: values.String(values.StrRejected),
		},
		{
			Text: values.String(values.StrAbandoned),
		},
	}, values.ProposalDropdownGroup, 1)

	return pg
}

// OnNavigatedTo is called when the page is about to be displayed and
// may be used to initialize page features that are only relevant when
// the page is displayed.
// Part of the load.Page interface.
func (pg *ProposalsPage) OnNavigatedTo() {
	pg.ctx, pg.ctxCancel = context.WithCancel(context.TODO())
	pg.listenForSyncNotifications()
	pg.fetchProposals()
	pg.isSyncing = pg.multiWallet.Politeia.IsSyncing()
}

func (pg *ProposalsPage) fetchProposals() {
	newestFirst := pg.orderDropDown.SelectedIndex() == 0

	proposalFilter := libwallet.ProposalCategoryAll
	switch pg.categoryDropDown.SelectedIndex() {
	case 1:
		proposalFilter = libwallet.ProposalCategoryApproved
	case 2:
		proposalFilter = libwallet.ProposalCategoryRejected
	case 3:
		proposalFilter = libwallet.ProposalCategoryAbandoned
	}

	proposalItems := components.LoadProposals(proposalFilter, newestFirst, pg.Load)

	// group 'In discussion' and 'Active' proposals into under review
	listItems := make([]*components.ProposalItem, 0)
	for _, item := range proposalItems {
		if item.Proposal.Category == libwallet.ProposalCategoryPre ||
			item.Proposal.Category == libwallet.ProposalCategoryActive {
			listItems = append(listItems, item)
		}
	}

	pg.proposalMu.Lock()
	pg.proposalItems = proposalItems
	if proposalFilter == libwallet.ProposalCategoryAll {
		pg.proposalItems = listItems
	}
	pg.proposalMu.Unlock()
}

// HandleUserInteractions is called just before Layout() to determine
// if any user interaction recently occurred on the page and may be
// used to update the page's UI components shortly before they are
// displayed.
// Part of the load.Page interface.
func (pg *ProposalsPage) HandleUserInteractions() {
	for pg.categoryDropDown.Changed() {
		pg.fetchProposals()
	}

	for pg.orderDropDown.Changed() {
		pg.fetchProposals()
	}

	pg.searchEditor.EditorIconButtonEvent = func() {
		//TODO: Proposals search functionality
	}

	if clicked, selectedItem := pg.proposalsList.ItemClicked(); clicked {
		pg.proposalMu.Lock()
		selectedProposal := pg.proposalItems[selectedItem].Proposal
		pg.proposalMu.Unlock()

		pg.ParentNavigator().Display(NewProposalDetailsPage(pg.Load, &selectedProposal))
	}

	for pg.syncButton.Clicked() {
		go pg.multiWallet.Politeia.Sync()
		pg.isSyncing = true

		//Todo: check after 1min if sync does not start, set isSyncing to false and cancel sync
	}

	if pg.infoButton.Button.Clicked() {
		infoModal := modal.NewInfoModal(pg.Load).
			Title(values.String(values.StrProposal)).
			Body(values.String(values.StrOffChainVote)).
			SetCancelable(true).
			PositiveButton(values.String(values.StrGotIt), func(isChecked bool) bool {
				return true
			})
		pg.ParentWindow().ShowModal(infoModal)
	}

	if pg.syncCompleted {
		time.AfterFunc(time.Second*3, func() {
			pg.syncCompleted = false
			pg.ParentWindow().Reload()
		})
	}

	decredmaterial.DisplayOneDropdown(pg.orderDropDown, pg.categoryDropDown)

	for pg.infoButton.Button.Clicked() {
		//TODO: proposal info modal
	}
}

// OnNavigatedFrom is called when the page is about to be removed from
// the displayed window. This method should ideally be used to disable
// features that are irrelevant when the page is NOT displayed.
// NOTE: The page may be re-displayed on the app's window, in which case
// OnNavigatedTo() will be called again. This method should not destroy UI
// components unless they'll be recreated in the OnNavigatedTo() method.
// Part of the load.Page interface.
func (pg *ProposalsPage) OnNavigatedFrom() {
	pg.ctxCancel()
}

// Layout draws the page UI components into the provided layout context
// to be eventually drawn on screen.
// Part of the load.Page interface.
func (pg *ProposalsPage) Layout(gtx C) D {
	if pg.Load.GetCurrentAppWidth() <= gtx.Dp(values.StartMobileView) {
		return pg.layoutMobile(gtx)
	}
	return pg.layoutDesktop(gtx)
}

func (pg *ProposalsPage) layoutDesktop(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(pg.layoutSectionHeader),
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding60}.Layout(gtx, pg.layoutContent)
					}),
					//TODO: temp removal till after V1
					// layout.Expanded(func(gtx C) D {
					// 	gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding150)
					// 	gtx.Constraints.Min.X = gtx.Constraints.Max.X

					// 	card := pg.Theme.Card()
					// 	card.Radius = decredmaterial.Radius(8)
					// 	return card.Layout(gtx, func(gtx C) D {
					// 		return layout.Inset{
					// 			Left:   values.MarginPadding10,
					// 			Right:  values.MarginPadding10,
					// 			Top:    values.MarginPadding2,
					// 			Bottom: values.MarginPadding2,
					// 		}.Layout(gtx, pg.searchEditor.Layout)
					// 	})
					// }),
					layout.Expanded(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.E.Layout(gtx, func(gtx C) D {
							card := pg.Theme.Card()
							card.Radius = decredmaterial.Radius(8)
							return card.Layout(gtx, func(gtx C) D {
								return layout.UniformInset(values.MarginPadding8).Layout(gtx, pg.layoutSyncSection)
							})
						})
					}),
					layout.Expanded(func(gtx C) D {
						return pg.orderDropDown.Layout(gtx, 45, true)
					}),
					layout.Expanded(func(gtx C) D {
						return pg.categoryDropDown.Layout(gtx, pg.orderDropDown.Width+41, true)
					}),
				)
			})
		}),
	)
}

func (pg *ProposalsPage) layoutMobile(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, pg.layoutSectionHeader)
		}),
		layout.Flexed(1, func(gtx C) D {
			return layout.Inset{Top: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
				return layout.Stack{}.Layout(gtx,
					layout.Expanded(func(gtx C) D {
						return layout.Inset{Top: values.MarginPadding60}.Layout(gtx, pg.layoutContent)
					}),
					//TODO: temp removal till after V1
					// layout.Expanded(func(gtx C) D {
					// 	gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding150)
					// 	gtx.Constraints.Min.X = gtx.Constraints.Max.X

					// 	card := pg.Theme.Card()
					// 	card.Radius = decredmaterial.Radius(8)
					// 	return card.Layout(gtx, func(gtx C) D {
					// 		return layout.Inset{
					// 			Left:   values.MarginPadding10,
					// 			Right:  values.MarginPadding10,
					// 			Top:    values.MarginPadding2,
					// 			Bottom: values.MarginPadding2,
					// 		}.Layout(gtx, pg.searchEditor.Layout)
					// 	})
					// }),
					layout.Expanded(func(gtx C) D {
						gtx.Constraints.Min.X = gtx.Constraints.Max.X
						return layout.E.Layout(gtx, func(gtx C) D {
							card := pg.Theme.Card()
							card.Radius = decredmaterial.Radius(8)
							return layout.Inset{Right: values.MarginPadding10}.Layout(gtx, func(gtx C) D {
								return card.Layout(gtx, func(gtx C) D {
									return layout.UniformInset(values.MarginPadding8).Layout(gtx, pg.layoutSyncSection)
								})
							})
						})
					}),
					layout.Expanded(func(gtx C) D {
						return pg.orderDropDown.Layout(gtx, 55, true)
					}),
					layout.Expanded(func(gtx C) D {
						return pg.categoryDropDown.Layout(gtx, pg.orderDropDown.Width+51, true)
					}),
				)
			})
		}),
	)
}

func (pg *ProposalsPage) layoutContent(gtx C) D {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx C) D {
			pg.proposalMu.Lock()
			proposalItems := pg.proposalItems
			pg.proposalMu.Unlock()

			return pg.Theme.List(pg.listContainer).Layout(gtx, 1, func(gtx C, i int) D {
				return layout.Inset{Right: values.MarginPadding2}.Layout(gtx, func(gtx C) D {
					return pg.Theme.Card().Layout(gtx, func(gtx C) D {
						if len(proposalItems) == 0 {
							return components.LayoutNoProposalsFound(gtx, pg.Load, pg.isSyncing, int32(pg.categoryDropDown.SelectedIndex()))
						}
						return pg.proposalsList.Layout(gtx, len(proposalItems), func(gtx C, i int) D {
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
								layout.Rigid(func(gtx C) D {
									return components.ProposalsList(pg.ParentWindow(), gtx, pg.Load, proposalItems[i])
								}),
								layout.Rigid(func(gtx C) D {
									return pg.Theme.Separator().Layout(gtx)
								}),
							)
						})
					})
				})
			})
		}),
	)
}

func (pg *ProposalsPage) layoutSyncSection(gtx C) D {
	if pg.isSyncing {
		return pg.layoutIsSyncingSection(gtx)
	} else if pg.syncCompleted {
		return pg.updatedIcon.Layout(gtx, values.MarginPadding20)
	}
	return pg.layoutStartSyncSection(gtx)
}

func (pg *ProposalsPage) layoutIsSyncingSection(gtx C) D {
	gtx.Constraints.Max.X = gtx.Dp(values.MarginPadding24)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	loader := material.Loader(pg.Theme.Base)
	loader.Color = pg.Theme.Color.Gray1
	return loader.Layout(gtx)
}

func (pg *ProposalsPage) layoutStartSyncSection(gtx C) D {
	// TODO: use decredmaterial clickable
	return material.Clickable(gtx, pg.syncButton, pg.Theme.Icons.Restore.Layout24dp)
}

func (pg *ProposalsPage) layoutSectionHeader(gtx C) D {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(pg.Theme.Label(values.TextSize20, values.String(values.StrProposal)).Layout), // Do we really need to display the title? nav is proposals already
				layout.Rigid(pg.infoButton.Layout),
			)
		}),
		layout.Flexed(1, func(gtx C) D {
			body := func(gtx C) D {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						var text string
						if pg.isSyncing {
							text = values.String(values.StrSyncingState)
						} else if pg.syncCompleted {
							text = values.String(values.StrUpdated)
						} else {
							text = values.String(values.StrUpdated) + " " + components.TimeAgo(pg.multiWallet.Politeia.GetLastSyncedTimeStamp())
						}

						lastUpdatedInfo := pg.Theme.Label(values.TextSize10, text)
						lastUpdatedInfo.Color = pg.Theme.Color.GrayText2
						if pg.syncCompleted {
							lastUpdatedInfo.Color = pg.Theme.Color.Success
						}

						return layout.Inset{Top: values.MarginPadding2}.Layout(gtx, lastUpdatedInfo.Layout)
					}),
				)
			}
			return layout.E.Layout(gtx, body)
		}),
	)
}

func (pg *ProposalsPage) listenForSyncNotifications() {
	if pg.ProposalNotificationListener != nil {
		return
	}
	pg.ProposalNotificationListener = listeners.NewProposalNotificationListener()
	err := pg.WL.MultiWallet.Politeia.AddNotificationListener(pg.ProposalNotificationListener, ProposalsPageID)
	if err != nil {
		log.Errorf("Error adding politeia notification listener: %v", err)
		return
	}

	go func() {
		for {
			select {
			case n := <-pg.ProposalNotifChan:
				if n.ProposalStatus == wallet.Synced {
					pg.syncCompleted = true
					pg.isSyncing = false

					pg.fetchProposals()
					pg.ParentWindow().Reload()
				}
			case <-pg.ctx.Done():
				pg.WL.MultiWallet.Politeia.RemoveNotificationListener(ProposalsPageID)
				close(pg.ProposalNotifChan)
				pg.ProposalNotificationListener = nil

				return
			}
		}
	}()
}
