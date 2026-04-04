//go:build gtk4

package main

import (
	"context"
	_ "embed"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ka2n/way-island/internal/gtkmini"
	"github.com/ka2n/way-island/internal/socket"
)

//go:embed style.css
var styleCSS string

const hoverCloseDelayMS = 160
const widthAnimationDuration = 200 * time.Millisecond
const shellClosedWidth = 200
const shellExpandedWidth = 340

var gtkSessionFocuser *sessionFocuser

type gtkUI struct {
	app               *gtkmini.Application
	window            *gtkmini.Window
	shell             *gtkmini.Widget
	root              *gtkmini.Widget
	pill              *gtkmini.Widget
	revealer          *gtkmini.Widget
	stack             *gtkmini.Widget
	listPage          *gtkmini.Widget
	detailPage        *gtkmini.Widget
	cssProvider       *gtkmini.CSSProvider
	sessionsPayload   string
	selectedSessionID string
	cssData           string
	shouldQuit        bool
	panelView         int
	stackView         int
	pendingPanelView  int
	panelUpdateSource gtkmini.SourceID
	hoverCloseSource  gtkmini.SourceID
	widthAnimSource   gtkmini.SourceID
	widthAnimFrom     int
	widthAnimTo       int
	widthAnimCurrent  int
	widthAnimStart    time.Time
}

func newGTKUI() *gtkUI {
	return &gtkUI{
		panelView:        panelViewClosed,
		stackView:        panelViewList,
		pendingPanelView: -1,
	}
}

func (ui *gtkUI) cancelHoverClose() {
	if ui.hoverCloseSource == 0 {
		return
	}
	gtkmini.SourceRemove(ui.hoverCloseSource)
	ui.hoverCloseSource = 0
}

func easeOutCubic(t float64) float64 {
	inverse := 1.0 - t
	return 1.0 - (inverse * inverse * inverse)
}

func (ui *gtkUI) targetShellWidth() int {
	if ui.panelView != panelViewClosed && strings.TrimSpace(ui.sessionsPayload) != "" {
		return shellExpandedWidth
	}
	return shellClosedWidth
}

func (ui *gtkUI) stopWidthAnimation() {
	if ui.widthAnimSource == 0 {
		return
	}
	gtkmini.SourceRemove(ui.widthAnimSource)
	ui.widthAnimSource = 0
}

func (ui *gtkUI) updateShellWidth(animate bool) {
	if ui.shell == nil {
		return
	}

	targetWidth := ui.targetShellWidth()
	currentWidth := ui.shell.Width()
	if ui.widthAnimSource != 0 {
		currentWidth = ui.widthAnimCurrent
	}

	ui.stopWidthAnimation()

	if !animate || currentWidth <= 0 || currentWidth == targetWidth {
		ui.widthAnimCurrent = targetWidth
		ui.shell.SetSizeRequest(targetWidth, -1)
		ui.shell.QueueResize()
		return
	}

	ui.widthAnimFrom = currentWidth
	ui.widthAnimTo = targetWidth
	ui.widthAnimCurrent = currentWidth
	ui.widthAnimStart = time.Now()
	ui.widthAnimSource = gtkmini.TimeoutAdd(16, func() bool {
		elapsed := time.Since(ui.widthAnimStart)
		progress := float64(elapsed) / float64(widthAnimationDuration)
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}

		eased := easeOutCubic(progress)
		width := int(float64(ui.widthAnimFrom) + float64(ui.widthAnimTo-ui.widthAnimFrom)*eased + 0.5)
		ui.widthAnimCurrent = width
		ui.shell.SetSizeRequest(width, -1)
		ui.shell.QueueResize()

		if progress >= 1 {
			ui.widthAnimSource = 0
			ui.widthAnimCurrent = ui.widthAnimTo
			ui.shell.SetSizeRequest(ui.widthAnimTo, -1)
			return false
		}
		return true
	})
}

func (ui *gtkUI) schedulePanelView(panelView int) {
	if ui.pendingPanelView == panelView {
		return
	}

	ui.pendingPanelView = panelView
	if ui.panelUpdateSource != 0 {
		gtkmini.SourceRemove(ui.panelUpdateSource)
		ui.panelUpdateSource = 0
	}

	ui.panelUpdateSource = gtkmini.IdleAdd(func() {
		ui.panelUpdateSource = 0
		ui.pendingPanelView = -1
		if ui.panelView == panelView {
			return
		}

		ui.panelView = panelView
		ui.rebuildUI(ui.sessionsPayload)
	})
}

func (ui *gtkUI) setStackView(panelView int) {
	if ui.stack == nil {
		return
	}

	if panelView == panelViewDetail {
		if ui.stackView != panelViewDetail {
			ui.stack.StackSetTransitionType(gtkmini.StackTransitionSlideLeft)
		}
		ui.stack.StackSetVisibleChildName("detail")
		ui.stackView = panelViewDetail
		return
	}

	if ui.stackView != panelViewList {
		ui.stack.StackSetTransitionType(gtkmini.StackTransitionSlideRight)
	}
	ui.stack.StackSetVisibleChildName("list")
	ui.stackView = panelViewList
}

func (ui *gtkUI) onRevealerChildRevealedChanged() {
	if ui.revealer == nil {
		return
	}
	if ui.revealer.RevealerGetRevealChild() {
		return
	}
	if ui.revealer.RevealerGetChildRevealed() {
		return
	}

	ui.revealer.SetVisible(false)
	ui.setStackView(panelViewList)
	if ui.shell != nil {
		ui.updateShellWidth(false)
		ui.shell.QueueResize()
	}
}

func (ui *gtkUI) openDetail(sessionID string) {
	if sessionID == "" {
		return
	}
	ui.cancelHoverClose()
	ui.selectedSessionID = sessionID
	ui.schedulePanelView(panelViewDetail)
}

func (ui *gtkUI) openPrimaryDetail() {
	sessions := parsePayloadSessions(ui.sessionsPayload)
	if len(sessions) == 0 {
		return
	}
	ui.openDetail(sessions[0].ID)
}

func (ui *gtkUI) openList() {
	if strings.TrimSpace(ui.sessionsPayload) == "" {
		return
	}
	ui.cancelHoverClose()
	ui.schedulePanelView(panelViewList)
}

func (ui *gtkUI) closePanel() {
	ui.cancelHoverClose()
	ui.schedulePanelView(panelViewClosed)
}

func (ui *gtkUI) onHoverEnter() {
	ui.cancelHoverClose()
	if strings.TrimSpace(ui.sessionsPayload) == "" {
		return
	}
	if ui.panelView != panelViewClosed {
		return
	}

	ui.openList()
}

func (ui *gtkUI) onHoverLeave() {
	if ui.panelView == panelViewClosed {
		return
	}

	ui.cancelHoverClose()
	ui.hoverCloseSource = gtkmini.TimeoutAdd(hoverCloseDelayMS, func() bool {
		ui.hoverCloseSource = 0
		ui.closePanel()
		return false
	})
}

func (ui *gtkUI) buildCountBadge(group string, count int) *gtkmini.Widget {
	badge := gtkmini.NewBox(gtkmini.OrientationHorizontal, 6)
	badge.AddCSSClass("island-count-badge")

	square := gtkmini.NewBox(gtkmini.OrientationHorizontal, 0)
	square.AddCSSClass("island-count-square")
	square.AddCSSClass(group)
	badge.Append(square)

	label := gtkmini.NewLabel(itoa(count))
	label.AddCSSClass("island-count-label")
	label.SetVAlign(gtkmini.AlignCenter)
	badge.Append(label)

	return badge
}

func sessionGroup(state string) string {
	switch state {
	case "waiting":
		return "waiting"
	case "working":
		return "working"
	default:
		return "other"
	}
}

func (ui *gtkUI) rebuildPill(sessions []payloadSession, pill pillViewModel) {
	if ui.pill == nil {
		return
	}

	ui.pill.ClearBoxChildren()

	waitingCount := 0
	workingCount := 0
	otherCount := 0
	for _, session := range sessions {
		switch sessionGroup(session.State) {
		case "waiting":
			waitingCount++
		case "working":
			workingCount++
		default:
			otherCount++
		}
	}

	status := gtkmini.NewBox(gtkmini.OrientationHorizontal, 0)
	status.AddCSSClass("island-status")
	status.AddCSSClass(pill.StateClass)
	status.SetVAlign(gtkmini.AlignCenter)

	summary := gtkmini.NewBox(gtkmini.OrientationVertical, 0)
	summary.AddCSSClass("island-summary")
	summary.SetHexpand(true)
	if pill.Clickable {
		summary.ConnectClick(func() {
			ui.openPrimaryDetail()
		})
	}

	summaryContent := gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	summaryContent.AddCSSClass("island-summary-content")
	summary.Append(summaryContent)
	summaryContent.Append(status)

	title := gtkmini.NewLabel(pill.Title)
	title.AddCSSClass("island-title")
	title.SetHexpand(true)
	title.SetHAlign(gtkmini.AlignFill)
	title.SetVAlign(gtkmini.AlignCenter)
	title.LabelSetEllipsizeEnd()
	title.LabelSetXAlign(0)
	summaryContent.Append(title)

	ui.pill.Append(summary)

	if len(sessions) == 0 {
		return
	}

	counts := gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	counts.AddCSSClass("island-counts")
	counts.SetHAlign(gtkmini.AlignEnd)
	counts.SetVAlign(gtkmini.AlignCenter)
	hasVisibleCount := false
	if waitingCount > 0 {
		counts.Append(ui.buildCountBadge("waiting", waitingCount))
		hasVisibleCount = true
	}
	if workingCount > 0 {
		counts.Append(ui.buildCountBadge("working", workingCount))
		hasVisibleCount = true
	}
	if otherCount > 0 {
		counts.Append(ui.buildCountBadge("other", otherCount))
		hasVisibleCount = true
	}
	if hasVisibleCount {
		ui.pill.Append(counts)
	}
}

func (ui *gtkUI) buildSessionRow(session payloadSession) *gtkmini.Widget {
	rowShell := gtkmini.NewBox(gtkmini.OrientationVertical, 0)
	rowShell.AddCSSClass("session-row-shell")

	row := gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	row.AddCSSClass("session-row")
	rowShell.Append(row)
	if session.LastUserMessage != "" {
		rowShell.SetTooltipText(session.LastUserMessage)
		row.SetTooltipText(session.LastUserMessage)
	}
	row.ConnectClick(func() {
		ui.openDetail(session.ID)
	})

	dot := gtkmini.NewBox(gtkmini.OrientationHorizontal, 0)
	dot.AddCSSClass("island-status")
	dot.AddCSSClass(statusClass(session.State))
	dot.SetVAlign(gtkmini.AlignCenter)
	row.Append(dot)

	textBox := gtkmini.NewBox(gtkmini.OrientationVertical, 2)
	textBox.SetHexpand(true)

	title := gtkmini.NewLabel(session.Name)
	title.AddCSSClass("session-row-title")
	title.LabelSetXAlign(0)
	title.LabelSetEllipsizeEnd()
	title.LabelSetMaxWidthChars(30)
	textBox.Append(title)

	statusText := gtkmini.NewLabel(actionOrStatusLabel(session.Action, session.State))
	statusText.AddCSSClass("session-row-status")
	statusText.LabelSetXAlign(0)
	statusText.LabelSetEllipsizeEnd()
	statusText.LabelSetMaxWidthChars(48)
	textBox.Append(statusText)

	row.Append(textBox)

	chevron := gtkmini.NewImageFromIconName("go-next-symbolic")
	chevron.AddCSSClass("session-row-chevron")
	chevron.SetVAlign(gtkmini.AlignCenter)
	row.Append(chevron)

	return rowShell
}

func (ui *gtkUI) rebuildList(sessions []payloadSession) {
	if ui.listPage == nil {
		return
	}

	ui.listPage.ClearBoxChildren()

	header := gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	header.AddCSSClass("detail-header")

	title := gtkmini.NewLabel("Sessions")
	title.AddCSSClass("detail-title")
	title.LabelSetXAlign(0)
	header.Append(title)
	ui.listPage.Append(header)

	for _, session := range sessions {
		ui.listPage.Append(ui.buildSessionRow(session))
	}
}

func (ui *gtkUI) rebuildSelectedDetail(sessions []payloadSession) bool {
	if ui.detailPage == nil || ui.selectedSessionID == "" {
		return false
	}

	ui.detailPage.ClearBoxChildren()

	detail := buildDetailViewModel(sessions, ui.selectedSessionID)
	if detail == nil {
		return false
	}

	header := gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	header.AddCSSClass("detail-header")

	back := gtkmini.NewBox(gtkmini.OrientationHorizontal, 0)
	back.AddCSSClass("detail-back-button")
	back.SetTooltipText("Back")
	back.ConnectClick(func() {
		ui.schedulePanelView(panelViewList)
	})

	backContent := gtkmini.NewBox(gtkmini.OrientationHorizontal, 0)
	backContent.AddCSSClass("detail-back-button-content")
	back.Append(backContent)

	backIcon := gtkmini.NewImageFromIconName("go-previous-symbolic")
	backIcon.AddCSSClass("session-row-chevron")
	backContent.Append(backIcon)
	header.Append(back)

	headerTitle := gtkmini.NewLabel("Session detail")
	headerTitle.AddCSSClass("detail-title")
	headerTitle.LabelSetXAlign(0)
	header.Append(headerTitle)
	ui.detailPage.Append(header)

	card := gtkmini.NewBox(gtkmini.OrientationVertical, 10)
	card.AddCSSClass("detail-card")
	ui.detailPage.Append(card)

	cardContent := gtkmini.NewBox(gtkmini.OrientationVertical, 10)
	cardContent.AddCSSClass("detail-card-content")
	card.Append(cardContent)

	detailName := gtkmini.NewLabel(detail.Title)
	detailName.AddCSSClass("detail-session-title")
	detailName.LabelSetXAlign(0)
	detailName.LabelSetWrap(true)
	cardContent.Append(detailName)

	stateRow := gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	cardContent.Append(stateRow)

	dot := gtkmini.NewBox(gtkmini.OrientationHorizontal, 0)
	dot.AddCSSClass("island-status")
	dot.AddCSSClass(detail.StateClass)
	dot.SetVAlign(gtkmini.AlignCenter)
	stateRow.Append(dot)

	stateLabel := gtkmini.NewLabel(detail.StatusLabel)
	stateLabel.AddCSSClass("detail-state-label")
	stateLabel.LabelSetXAlign(0)
	stateRow.Append(stateLabel)

	if detail.BodyText != "" {
		body := gtkmini.NewLabel(detail.BodyText)
		body.AddCSSClass("detail-body")
		body.LabelSetXAlign(0)
		body.LabelSetWrap(true)
		cardContent.Append(body)
	}

	sessionID := detail.SessionID
	focusButton := gtkmini.NewButtonWithLabel("Open session")
	focusButton.AddCSSClass("detail-focus-button")
	focusButton.ConnectButtonClicked(func() {
		ui.closePanel()
		if gtkSessionFocuser != nil {
			triggerSessionFocus(gtkSessionFocuser, sessionID)
		}
	})
	cardContent.Append(focusButton)

	return true
}

func (ui *gtkUI) rebuildDetail(sessions []payloadSession) {
	if ui.stack == nil {
		return
	}

	ui.rebuildList(sessions)

	if ui.panelView == panelViewDetail && ui.rebuildSelectedDetail(sessions) {
		ui.setStackView(panelViewDetail)
		return
	}

	if ui.panelView == panelViewDetail && len(sessions) > 0 {
		ui.openDetail(sessions[0].ID)
		if ui.rebuildSelectedDetail(sessions) {
			ui.setStackView(panelViewDetail)
			return
		}
	}

	ui.setStackView(panelViewList)
}

func (ui *gtkUI) rebuildUI(payload string) {
	if ui.root == nil || ui.shell == nil {
		return
	}

	animateWidth := ui.shell.Width() > 0
	sessions := parsePayloadSessions(payload)
	vm := buildOverlayViewModel(payload, ui.panelView, ui.selectedSessionID)

	ui.shell.AddCSSClass("island-pill")
	if !vm.HasSessions {
		ui.shell.RemoveCSSClass("expanded")
		ui.panelView = panelViewClosed
		ui.pendingPanelView = -1
		if ui.panelUpdateSource != 0 {
			gtkmini.SourceRemove(ui.panelUpdateSource)
			ui.panelUpdateSource = 0
		}
		ui.cancelHoverClose()
		ui.selectedSessionID = ""
	}

	ui.rebuildPill(sessions, vm.Pill)

	if ui.panelView != panelViewClosed && vm.HasSessions {
		ui.shell.AddCSSClass("expanded")
		if ui.panelView == panelViewDetail {
			ui.shell.AddCSSClass("detail-view")
		} else {
			ui.shell.RemoveCSSClass("detail-view")
		}
		ui.rebuildDetail(sessions)
		ui.revealer.SetVisible(true)
		ui.revealer.RevealerSetRevealChild(true)
		ui.updateShellWidth(animateWidth)
		return
	}

	ui.shell.RemoveCSSClass("expanded")
	ui.shell.RemoveCSSClass("detail-view")
	ui.setStackView(panelViewList)
	ui.revealer.RevealerSetRevealChild(false)
	ui.shell.QueueResize()
	ui.updateShellWidth(animateWidth)
}

func (ui *gtkUI) buildShellWidget() *gtkmini.Widget {
	ui.shell = gtkmini.NewShell()

	ui.root = gtkmini.NewBox(gtkmini.OrientationVertical, 0)
	gtkmini.ShellSetChild(ui.shell, ui.root)

	ui.pill = gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	ui.pill.AddCSSClass("island-content")
	ui.pill.SetHexpand(true)
	ui.root.Append(ui.pill)

	ui.revealer = gtkmini.NewRevealer()
	ui.revealer.RevealerSetTransitionType(gtkmini.RevealerTransitionSlideDown)
	ui.revealer.RevealerSetTransitionDuration(200)
	ui.revealer.RevealerSetRevealChild(false)
	ui.revealer.SetVisible(false)
	ui.revealer.ConnectNotifyChildRevealed(func() {
		ui.onRevealerChildRevealedChanged()
	})
	ui.root.Append(ui.revealer)

	ui.stack = gtkmini.NewStack()
	ui.stack.AddCSSClass("island-detail")
	ui.stack.StackSetTransitionDuration(180)
	ui.stack.StackSetTransitionType(gtkmini.StackTransitionSlideLeftRight)
	ui.stack.StackSetHHomogeneous(false)
	ui.stack.StackSetVHomogeneous(false)
	ui.stack.StackSetInterpolateSize(true)
	ui.revealer.RevealerSetChild(ui.stack)

	ui.listPage = gtkmini.NewBox(gtkmini.OrientationVertical, 2)
	ui.listPage.AddCSSClass("detail-page")
	ui.stack.StackAddNamed(ui.listPage, "list")

	ui.detailPage = gtkmini.NewBox(gtkmini.OrientationVertical, 2)
	ui.detailPage.AddCSSClass("detail-page")
	ui.stack.StackAddNamed(ui.detailPage, "detail")
	ui.setStackView(panelViewList)

	ui.shell.ConnectHover(func() {
		ui.onHoverEnter()
	}, func() {
		ui.onHoverLeave()
	})

	ui.rebuildUI(ui.sessionsPayload)
	return ui.shell
}

func (ui *gtkUI) onActivate() {
	ui.window = gtkmini.NewApplicationWindow(ui.app)
	ui.cssProvider = gtkmini.NewCSSProvider()
	if ui.cssData != "" {
		ui.cssProvider.LoadFromString(ui.cssData)
	}

	ui.window.SetTitle("way-island")
	ui.window.SetResizable(false)
	ui.window.SetDecorated(false)
	gtkmini.AddProviderForDisplay(ui.window.Widget(), ui.cssProvider)

	shell := ui.buildShellWidget()
	if ui.shouldQuit {
		ui.app.Quit()
		return
	}

	ui.window.Widget().AddCSSClass("way-island-window")
	ui.window.Widget().RemoveCSSClass("background")
	ui.window.Widget().RemoveCSSClass("solid-csd")
	ui.window.SetChild(shell)
	gtkmini.LayerShellConfigureTop(ui.window)
	ui.window.Present()
}

func (ui *gtkUI) setCSS(css string) {
	ui.cssData = css
}

func (ui *gtkUI) scheduleSessionsPayloadUpdate(payload string) {
	gtkmini.IdleAdd(func() {
		ui.sessionsPayload = payload
		ui.rebuildUI(ui.sessionsPayload)
	})
}

func (ui *gtkUI) scheduleCSSUpdate(css string) {
	gtkmini.IdleAdd(func() {
		ui.cssData = css
		if ui.cssProvider != nil {
			ui.cssProvider.LoadFromString(ui.cssData)
		}
	})
}

func (ui *gtkUI) scheduleApplicationQuit() {
	gtkmini.IdleAdd(func() {
		ui.shouldQuit = true
		if ui.app != nil {
			ui.app.Quit()
		}
	})
}

func (ui *gtkUI) run() int {
	ui.app = gtkmini.NewApplication("com.github.ka2n.way-island")
	ui.app.ConnectActivate(func() {
		ui.onActivate()
	})

	status := ui.app.Run()
	ui.cancelHoverClose()
	ui.stopWidthAnimation()
	ui.widthAnimCurrent = 0
	if ui.panelUpdateSource != 0 {
		gtkmini.SourceRemove(ui.panelUpdateSource)
		ui.panelUpdateSource = 0
	}
	if ui.cssProvider != nil {
		ui.cssProvider.Unref()
		ui.cssProvider = nil
	}
	ui.app.Unref()

	ui.window = nil
	ui.shell = nil
	ui.root = nil
	ui.pill = nil
	ui.revealer = nil
	ui.stack = nil
	ui.listPage = nil
	ui.detailPage = nil
	ui.app = nil
	ui.panelView = panelViewClosed
	ui.stackView = panelViewList
	ui.pendingPanelView = -1
	ui.shouldQuit = false

	return status
}

func buildGTKWidgetForTest(payload string, panelView int, selectedSessionID string) *gtkmini.Widget {
	ui := newGTKUI()
	ui.sessionsPayload = payload
	ui.selectedSessionID = selectedSessionID
	ui.panelView = panelView
	return ui.buildShellWidget()
}

func runUI(ctx context.Context, updates <-chan socket.SessionUpdate, store *overlayModel) int {
	gtkSessionFocuser = newSessionFocuser(store)
	defer func() {
		gtkSessionFocuser = nil
	}()

	ui := newGTKUI()

	cssPaths, pathErr := resolveUserCSSPaths()
	if pathErr != nil {
		log.Printf("resolve user CSS paths: %v", pathErr)
	}

	appCSS := styleCSS
	if pathErr == nil {
		var err error
		appCSS, err = loadAppCSSFromPaths(styleCSS, cssPaths.StylePath, cssPaths.UserStylePath)
		if err != nil {
			log.Printf("load app CSS: %v", err)
		} else if css, err := loadUserStyleCSS(cssPaths.StylePath); err == nil && css != "" {
			log.Printf("load app CSS: using config style %s", cssPaths.StylePath)
		} else if css, err := loadUserStyleCSS(cssPaths.UserStylePath); err == nil && css != "" {
			log.Printf("load app CSS: merging user style %s", cssPaths.UserStylePath)
		} else {
			log.Printf("load app CSS: using builtin style.css")
		}
	}
	ui.setCSS(appCSS)

	go forwardUIUpdates(ctx, updates, store, func(payload string) {
		ui.scheduleSessionsPayloadUpdate(payload)
	})

	go func() {
		<-ctx.Done()
		ui.scheduleApplicationQuit()
	}()

	if pathErr == nil {
		go watchCSSChanges(ctx, cssPaths, appCSS, ui.scheduleCSSUpdate)
	}

	return ui.run()
}

func watchCSSChanges(ctx context.Context, cssPaths userCSSPaths, initialCSS string, apply func(string)) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("watch app CSS: create watcher: %v", err)
		return
	}
	defer watcher.Close()

	configDir := filepath.Dir(filepath.Dir(cssPaths.StylePath))
	watchPaths := cssWatchPaths(cssPaths)
	lastCSS := initialCSS
	var reloadTimer *time.Timer
	var reloadC <-chan time.Time
	var pendingReloadEvent string

	if err := addCSSWatchPaths(watcher, configDir, watchPaths); err != nil {
		log.Printf("watch app CSS: %v", err)
	}
	defer func() {
		if reloadTimer != nil {
			reloadTimer.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reloadC:
			reloadC = nil

			currentCSS, err := loadAppCSSFromPaths(styleCSS, cssPaths.StylePath, cssPaths.UserStylePath)
			if err != nil {
				log.Printf("watch app CSS: %v", err)
				continue
			}
			if currentCSS == lastCSS {
				continue
			}
			lastCSS = currentCSS

			log.Printf("watch app CSS: reloaded (event=%s)", pendingReloadEvent)
			apply(currentCSS)
		case err := <-watcher.Errors:
			if err != nil {
				log.Printf("watch app CSS: %v", err)
			}
		case event := <-watcher.Events:
			if !isRelevantCSSWatchEvent(event, watchPaths) {
				continue
			}

			watchPaths = cssWatchPaths(cssPaths)
			if err := ensureCSSWatchPaths(watcher, watchPaths); err != nil {
				log.Printf("watch app CSS: %v", err)
			}
			pendingReloadEvent = event.Op.String()
			resetReloadTimer(&reloadTimer, &reloadC, 75*time.Millisecond)
		}
	}
}

func resetReloadTimer(timer **time.Timer, timerC *<-chan time.Time, delay time.Duration) {
	if *timer == nil {
		*timer = time.NewTimer(delay)
		*timerC = (*timer).C
		return
	}

	if !(*timer).Stop() {
		select {
		case <-(*timer).C:
		default:
		}
	}
	(*timer).Reset(delay)
	*timerC = (*timer).C
}

func addCSSWatchPaths(watcher *fsnotify.Watcher, configDir string, watchPaths []string) error {
	if err := watcher.Add(configDir); err != nil {
		return err
	}
	return ensureCSSWatchPaths(watcher, watchPaths)
}

func ensureCSSWatchPaths(watcher *fsnotify.Watcher, watchPaths []string) error {
	for _, watchPath := range watchPaths {
		if err := ensureCSSDirWatch(watcher, filepath.Dir(watchPath)); err != nil {
			return err
		}
	}
	return nil
}

func ensureCSSDirWatch(watcher *fsnotify.Watcher, cssDir string) error {
	_, err := os.Stat(cssDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if isPathWatched(watcher, cssDir) {
		return nil
	}
	return watcher.Add(cssDir)
}

func isRelevantCSSWatchEvent(event fsnotify.Event, watchPaths []string) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}

	for _, watchPath := range watchPaths {
		if isRelevantSingleCSSWatchEvent(event, filepath.Dir(watchPath), filepath.Base(watchPath)) {
			return true
		}
	}

	return false
}

func isRelevantSingleCSSWatchEvent(event fsnotify.Event, cssDir, cssBase string) bool {
	if event.Name == cssPathJoin(cssDir, cssBase) {
		return true
	}

	if event.Name == cssDir {
		return true
	}

	if filepath.Dir(event.Name) == cssDir && filepath.Base(event.Name) == cssBase {
		return true
	}

	if filepath.Dir(event.Name) == filepath.Dir(cssDir) && filepath.Base(event.Name) == filepath.Base(cssDir) {
		return true
	}

	return false
}

func isPathWatched(watcher *fsnotify.Watcher, path string) bool {
	for _, watchedPath := range watcher.WatchList() {
		if watchedPath == path {
			return true
		}
	}
	return false
}

func cssPathJoin(dir, base string) string {
	return filepath.Join(dir, base)
}

func cssWatchPaths(paths userCSSPaths) []string {
	candidates := []string{
		paths.StylePath,
		resolveCSSWatchPath(paths.StylePath),
		paths.UserStylePath,
		resolveCSSWatchPath(paths.UserStylePath),
	}

	watchPaths := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		watchPaths = append(watchPaths, candidate)
	}

	return watchPaths
}

func resolveCSSWatchPath(cssPath string) string {
	resolvedPath, err := filepath.EvalSymlinks(cssPath)
	if err != nil {
		return cssPath
	}
	return resolvedPath
}
