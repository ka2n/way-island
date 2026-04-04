//go:build gtk4

package main

import (
	"context"
	_ "embed"
	"log"
	"os"
	"path/filepath"
	"strconv"
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
const listDetailAnimationDuration = 260 * time.Millisecond
const shellClosedWidth = 200
const shellExpandedWidth = 340
const animationScaleEnv = "WAY_ISLAND_ANIMATION_SCALE"
const animationDebugEnv = "WAY_ISLAND_DEBUG_ANIMATION"

const (
	listTransitionNone = iota
	listTransitionOpening
	listTransitionClosing
)

var gtkSessionFocuser *sessionFocuser
var animationDurationScale = mustLoadAnimationDurationScale()
var animationDebug = strings.TrimSpace(os.Getenv(animationDebugEnv)) == "1"

func debugAnimationLog(format string, args ...any) {
	if !animationDebug {
		return
	}
	log.Printf("animdbg: "+format, args...)
}

func mustLoadAnimationDurationScale() float64 {
	value := strings.TrimSpace(os.Getenv(animationScaleEnv))
	if value == "" {
		log.Printf("%s not set; using default animation scale 1", animationScaleEnv)
		return 1
	}

	scale, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Fatalf("%s must be a positive number: %v", animationScaleEnv, err)
	}
	if scale <= 0 {
		log.Fatalf("%s must be greater than 0, got %q", animationScaleEnv, value)
	}
	log.Printf("%s=%v", animationScaleEnv, scale)
	return scale
}

func scaledDuration(base time.Duration) time.Duration {
	scaled := time.Duration(float64(base) * animationDurationScale)
	if scaled < time.Millisecond {
		return time.Millisecond
	}
	return scaled
}

func (ui *gtkUI) currentDetailAnimationDuration() time.Duration {
	if ui != nil && ui.listDetailAnimating {
		return scaledDuration(listDetailAnimationDuration)
	}
	return scaledDuration(widthAnimationDuration)
}

type gtkUI struct {
	app                *gtkmini.Application
	backdropWindow     *gtkmini.Window
	window             *gtkmini.Window
	backdrop           *gtkmini.Widget
	shell              *gtkmini.Widget
	root               *gtkmini.Widget
	pill               *gtkmini.Widget
	detailHost         *gtkmini.Widget
	closingHost        *gtkmini.Widget
	slide              *gtkmini.Widget
	listPage           *gtkmini.Widget
	detailPage         *gtkmini.Widget
	closingListPage    *gtkmini.Widget
	cssProvider        *gtkmini.CSSProvider
	sessionsPayload    string
	selectedSessionID  string
	cssData            string
	shouldQuit         bool
	panelView          int
	stackView          int
	pendingPanelView   int
	panelUpdateSource  gtkmini.SourceID
	hoverCloseSource   gtkmini.SourceID
	backdropAnimSource gtkmini.SourceID
	widthAnimSource    gtkmini.SourceID
	detailAnimSource   gtkmini.SourceID
	detailOpenSource   gtkmini.SourceID
	slideAnimSource    gtkmini.SourceID
	widthAnimFrom      int
	widthAnimTo        int
	widthAnimCurrent   int
	detailAnimFrom     int
	detailAnimTo       int
	slideAnimFrom      float64
	slideAnimTo        float64
	cachedListWidth    int
	cachedDetailWidth  int
	cachedListHeight   int
	cachedDetailHeight int
	closingActive      bool
	listDetailAnimating bool
	listTransitionMode int
	wasExpanded        bool
	panelPinned        bool
	showingDetail      bool
}

func newGTKUI() *gtkUI {
	debugAnimationLog("%s=1", animationDebugEnv)
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

func measureShellWidth(widget *gtkmini.Widget, cached *int, fallback int) int {
	if widget != nil {
		if width := widget.MeasureNaturalWidth() + 32; width > 0 {
			if cached != nil {
				*cached = width
			}
			return width
		}
	}
	if cached != nil && *cached > 0 {
		return *cached
	}
	return fallback
}

func (ui *gtkUI) closedShellWidth() int {
	return measureShellWidth(ui.pill, nil, shellClosedWidth)
}

func (ui *gtkUI) listShellWidth() int {
	return measureShellWidth(ui.listPage, &ui.cachedListWidth, ui.closedShellWidth())
}

func (ui *gtkUI) detailShellWidth() int {
	return measureShellWidth(ui.detailPage, &ui.cachedDetailWidth, ui.listShellWidth())
}

func (ui *gtkUI) frozenListShellWidth() int {
	return measureShellWidth(ui.closingListPage, &ui.cachedListWidth, ui.listShellWidth())
}

func (ui *gtkUI) measuredWidthForPanelView(panelView int) int {
	switch panelView {
	case panelViewClosed:
		return ui.closedShellWidth()
	case panelViewDetail:
		return ui.detailShellWidth()
	default:
		return ui.listShellWidth()
	}
}

func contentWidthForShellWidth(width int) int {
	contentWidth := width - 32
	if contentWidth < 0 {
		return 0
	}
	return contentWidth
}

func (ui *gtkUI) applyShellWidth(width int) {
	if ui.shell == nil {
		return
	}
	ui.widthAnimCurrent = width
	ui.shell.SetSizeRequest(width, -1)
	ui.shell.QueueResize()

	contentWidth := contentWidthForShellWidth(width)
	if ui.detailHost != nil {
		ui.detailHost.SetSizeRequest(contentWidth, -1)
		ui.detailHost.QueueResize()
	}
	if ui.closingHost != nil {
		ui.closingHost.SetSizeRequest(contentWidth, -1)
		ui.closingHost.QueueResize()
	}
}

func (ui *gtkUI) targetShellWidth() int {
	if ui.panelView == panelViewClosed || strings.TrimSpace(ui.sessionsPayload) == "" {
		targetWidth := ui.closedShellWidth()
		debugAnimationLog("targetShellWidth panel=%d closed=%d", ui.panelView, targetWidth)
		return targetWidth
	}

	targetWidth := ui.listShellWidth()
	if ui.panelView == panelViewDetail {
		targetWidth = ui.detailShellWidth()
	}
	debugAnimationLog("targetShellWidth panel=%d closed=%d list=%d detail=%d target=%d", ui.panelView, ui.closedShellWidth(), ui.cachedListWidth, ui.cachedDetailWidth, targetWidth)
	return targetWidth
}

func (ui *gtkUI) stopWidthAnimation() {
	if ui.widthAnimSource == 0 {
		return
	}
	gtkmini.SourceRemove(ui.widthAnimSource)
	ui.widthAnimSource = 0
}

func (ui *gtkUI) stopBackdropAnimation() {
	if ui.backdropAnimSource == 0 {
		return
	}
	gtkmini.SourceRemove(ui.backdropAnimSource)
	ui.backdropAnimSource = 0
}

func (ui *gtkUI) animateBackdropOpacity(from, to float64, hideWhenDone bool) {
	if ui.backdrop == nil || ui.backdropWindow == nil {
		return
	}

	ui.stopBackdropAnimation()
	ui.backdropWindow.Widget().SetVisible(true)
	ui.backdrop.SetOpacity(from)

	if from == to {
		ui.backdrop.SetOpacity(to)
		if hideWhenDone && to <= 0 {
			ui.backdropWindow.Widget().SetVisible(false)
		}
		return
	}

	start := time.Now()
	duration := scaledDuration(180 * time.Millisecond)
	ui.backdropAnimSource = gtkmini.TimeoutAdd(16, func() bool {
		progress := float64(time.Since(start)) / float64(duration)
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
		eased := easeOutCubic(progress)
		opacity := from + (to-from)*eased
		ui.backdrop.SetOpacity(opacity)
		if progress >= 1 {
			ui.backdropAnimSource = 0
			ui.backdrop.SetOpacity(to)
			if hideWhenDone && to <= 0 {
				ui.backdropWindow.Widget().SetVisible(false)
			}
			return false
		}
		return true
	})
}

func (ui *gtkUI) syncBackdropVisibility() {
	if ui.backdropWindow == nil {
		return
	}

	active := ui.panelPinned && ui.panelView != panelViewClosed && strings.TrimSpace(ui.sessionsPayload) != ""
	if ui.backdrop == nil {
		ui.backdropWindow.Widget().SetVisible(active)
		return
	}

	if active {
		ui.backdrop.AddCSSClass("active")
		ui.animateBackdropOpacity(0, 1, false)
		return
	}

	ui.backdrop.RemoveCSSClass("active")
	ui.animateBackdropOpacity(1, 0, true)
}

func (ui *gtkUI) stopDetailAnimation() {
	if ui.detailAnimSource == 0 {
		return
	}
	gtkmini.SourceRemove(ui.detailAnimSource)
	ui.detailAnimSource = 0
}

func (ui *gtkUI) stopSlideAnimation() {
	if ui.slideAnimSource == 0 {
		return
	}
	gtkmini.SourceRemove(ui.slideAnimSource)
	ui.slideAnimSource = 0
}

func (ui *gtkUI) resetLiveSlideToList() {
	if ui.slide == nil {
		return
	}
	ui.stopSlideAnimation()
	ui.listDetailAnimating = false
	ui.showingDetail = false
	ui.stackView = panelViewList
	ui.slide.SlideSetShowingDetail(false)
	ui.slide.SlideSetProgress(1)
	if ui.detailPage != nil {
		ui.detailPage.SetOpacity(0)
	}
}

func (ui *gtkUI) cancelPendingDetailOpen() {
	if ui.detailOpenSource == 0 {
		return
	}
	gtkmini.SourceRemove(ui.detailOpenSource)
	ui.detailOpenSource = 0
}

func (ui *gtkUI) animateDetailHeight(from, to int, hideWhenDone bool, animate bool) {
	if ui.detailHost == nil {
		return
	}
	debugAnimationLog("animateDetailHeight panel=%d from=%d to=%d animate=%v hide=%v host=%d", ui.panelView, from, to, animate, hideWhenDone, ui.detailHost.Height())

	ui.stopDetailAnimation()
	ui.cancelPendingDetailOpen()

	if from < 0 {
		from = 0
	}
	if to < 0 {
		to = 0
	}

	ui.detailAnimFrom = from
	ui.detailAnimTo = to
	ui.detailHost.SetVisible(true)

	if !animate || from == to {
		ui.detailHost.ClipSetHeight(to)
		ui.detailHost.QueueResize()
		ui.cachedDetailHeight = to
		if hideWhenDone && to == 0 {
			ui.detailHost.ClipSetHeight(0)
			ui.detailHost.QueueResize()
			ui.detailHost.SetVisible(false)
		}
		return
	}

	ui.detailHost.ClipSetHeight(from)
	start := time.Now()
	duration := ui.currentDetailAnimationDuration()
	ui.detailAnimSource = gtkmini.TimeoutAdd(16, func() bool {
		progress := float64(time.Since(start)) / float64(duration)
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
		eased := easeOutCubic(progress)
		height := int(float64(ui.detailAnimFrom) + float64(ui.detailAnimTo-ui.detailAnimFrom)*eased + 0.5)
		ui.detailHost.ClipSetHeight(height)
		ui.detailHost.QueueResize()
		if height > 0 {
			ui.cachedDetailHeight = height
		}
		if progress >= 1 {
			debugAnimationLog("animateDetailHeight done panel=%d final=%d", ui.panelView, ui.detailAnimTo)
			ui.detailAnimSource = 0
			ui.detailHost.ClipSetHeight(ui.detailAnimTo)
			if ui.detailAnimTo > 0 {
				ui.cachedDetailHeight = ui.detailAnimTo
			}
			if hideWhenDone && ui.detailAnimTo == 0 {
				ui.detailHost.ClipSetHeight(0)
				ui.detailHost.QueueResize()
				ui.detailHost.SetVisible(false)
			}
			return false
		}
		return true
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (ui *gtkUI) contentMeasureWidth() int {
	width := shellExpandedWidth - 32
	if ui.shell != nil {
		if shellWidth := ui.shell.Width(); shellWidth > 0 {
			width = shellWidth - 32
		}
	}
	if width < 120 {
		return 120
	}
	return width
}

func measureWidthFor(widget *gtkmini.Widget, fallback int) int {
	if widget == nil {
		return fallback
	}
	width := fallback
	if minWidth := widget.MeasureMinWidth(); minWidth > width {
		width = minWidth
	}
	return width
}

func (ui *gtkUI) measureCurrentPanelHeight() int {
	if ui.slide == nil {
		return 0
	}

	measureWidth := ui.contentMeasureWidth()
	debugAnimationLog("measure start panel=%d width=%d list_min=%d detail_min=%d cached_list=%d cached_detail=%d showing_detail=%v", ui.panelView, measureWidth, measureWidthFor(ui.listPage, 0), measureWidthFor(ui.detailPage, 0), ui.cachedListHeight, ui.cachedDetailHeight, ui.showingDetail)
	listPageHeight := 0
	detailPageHeight := 0
	if ui.listPage != nil {
		if height := ui.listPage.MeasureNaturalHeight(measureWidthFor(ui.listPage, measureWidth)); height > 0 {
			listPageHeight = height
			debugAnimationLog("measure list_page=%d", height)
		}
	}
	if ui.detailPage != nil {
		if height := ui.detailPage.MeasureNaturalHeight(measureWidthFor(ui.detailPage, measureWidth)); height > 0 {
			detailPageHeight = height
			debugAnimationLog("measure detail_page=%d", height)
		}
	}

	if ui.panelView == panelViewDetail {
		if detailPageHeight > 0 {
			ui.cachedDetailHeight = detailPageHeight
			debugAnimationLog("measure return detail page=%d", detailPageHeight)
			return detailPageHeight
		}
		if ui.cachedDetailHeight > 0 {
			debugAnimationLog("measure return detail cached=%d", ui.cachedDetailHeight)
			return ui.cachedDetailHeight
		}
	} else {
		if listPageHeight > 0 {
			ui.cachedListHeight = listPageHeight
			debugAnimationLog("measure return list page=%d", listPageHeight)
			return listPageHeight
		}
		if ui.cachedListHeight > 0 {
			debugAnimationLog("measure return list cached=%d", ui.cachedListHeight)
			return ui.cachedListHeight
		}
	}

	if ui.cachedDetailHeight > 0 {
		debugAnimationLog("measure return cached_detail=%d", ui.cachedDetailHeight)
		return ui.cachedDetailHeight
	}
	if ui.cachedListHeight > 0 {
		debugAnimationLog("measure return cached_list=%d", ui.cachedListHeight)
		return ui.cachedListHeight
	}
	debugAnimationLog("measure return default=180")
	return 180
}

func (ui *gtkUI) syncDetailHostHeight(animate bool) {
	if ui.detailHost == nil {
		return
	}
	debugAnimationLog("syncDetailHostHeight panel=%d animate=%v host_height=%d", ui.panelView, animate, ui.detailHost.Height())

	ui.stopDetailAnimation()
	ui.cancelPendingDetailOpen()

	currentHeight := ui.detailHost.Height()
	if currentHeight < 0 {
		currentHeight = 0
	}

	ui.detailHost.SetVisible(true)
	if currentHeight > 0 {
		ui.detailHost.ClipSetHeight(currentHeight)
	} else {
		ui.detailHost.ClipSetHeight(0)
	}
	ui.detailHost.QueueResize()

	if ui.panelView == panelViewDetail {
		targetHeight := ui.measureCurrentPanelHeight()
		if targetHeight > 0 {
			fromHeight := ui.detailHost.Height()
			if fromHeight <= 0 {
				fromHeight = currentHeight
			}
			debugAnimationLog("syncDetailHostHeight immediate detail from=%d to=%d", fromHeight, targetHeight)
			ui.animateDetailHeight(fromHeight, targetHeight, false, animate)
			return
		}
	}

	ui.scheduleDetailHostHeightSync(currentHeight, animate, 0)
}

func (ui *gtkUI) scheduleDetailHostHeightSync(currentHeight int, animate bool, attempt int) {
	ui.detailOpenSource = gtkmini.IdleAdd(func() {
		ui.detailOpenSource = 0
		if ui.detailHost == nil || ui.slide == nil || ui.panelView == panelViewClosed {
			return
		}
		debugAnimationLog("scheduleDetailHostHeightSync panel=%d animate=%v attempt=%d currentHeight=%d host=%d", ui.panelView, animate, attempt, currentHeight, ui.detailHost.Height())

		if ui.panelView == panelViewDetail && ui.detailPage != nil && ui.detailPage.MeasureNaturalHeight(measureWidthFor(ui.detailPage, ui.contentMeasureWidth())) == 0 && attempt < 6 {
			debugAnimationLog("scheduleDetailHostHeightSync retry detail natural height=0")
			ui.scheduleDetailHostHeightSync(currentHeight, animate, attempt+1)
			return
		}

		targetHeight := ui.measureCurrentPanelHeight()
		fromHeight := ui.detailHost.Height()
		if fromHeight <= 0 {
			fromHeight = currentHeight
		}
		ui.animateDetailHeight(fromHeight, targetHeight, false, animate)
	})
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
	debugAnimationLog("updateShellWidth panel=%d animate=%v current=%d target=%d source_active=%v", ui.panelView, animate, currentWidth, targetWidth, ui.widthAnimSource != 0)

	ui.stopWidthAnimation()

	if !animate || currentWidth <= 0 || currentWidth == targetWidth {
		debugAnimationLog("updateShellWidth immediate panel=%d width=%d", ui.panelView, targetWidth)
		ui.applyShellWidth(targetWidth)
		return
	}

	ui.widthAnimFrom = currentWidth
	ui.widthAnimTo = targetWidth
	ui.widthAnimCurrent = currentWidth
	start := time.Now()
	duration := scaledDuration(widthAnimationDuration)
	debugAnimationLog("updateShellWidth animate_start panel=%d from=%d to=%d duration=%s", ui.panelView, ui.widthAnimFrom, ui.widthAnimTo, duration)
	ui.widthAnimSource = gtkmini.TimeoutAdd(16, func() bool {
		progress := float64(time.Since(start)) / float64(duration)
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
		eased := easeOutCubic(progress)
		width := int(float64(ui.widthAnimFrom) + float64(ui.widthAnimTo-ui.widthAnimFrom)*eased + 0.5)
		ui.applyShellWidth(width)

		if progress >= 1 {
			ui.widthAnimSource = 0
			ui.applyShellWidth(ui.widthAnimTo)
			debugAnimationLog("updateShellWidth animate_done panel=%d final=%d", ui.panelView, ui.widthAnimTo)
			return false
		}
		return true
	})
}

func (ui *gtkUI) frozenListHeight() int {
	measureWidth := measureWidthFor(ui.closingListPage, ui.contentMeasureWidth())
	if ui.closingListPage != nil {
		if height := ui.closingListPage.MeasureNaturalHeight(measureWidth); height > 0 {
			ui.cachedListHeight = height
		}
	}
	if ui.cachedListHeight > 0 {
		return ui.cachedListHeight
	}
	return 180
}

func (ui *gtkUI) hideFrozenListHost() {
	if ui.closingHost == nil {
		return
	}
	ui.closingHost.ClipSetHeight(0)
	ui.closingHost.SetVisible(false)
}

func (ui *gtkUI) completeFrozenListOpen() {
	if ui.panelView != panelViewList {
		return
	}
	debugAnimationLog("completeFrozenListOpen")
	ui.listTransitionMode = listTransitionNone
	ui.closingActive = false
	ui.hideFrozenListHost()
	ui.wasExpanded = true
	ui.rebuildDetail(parsePayloadSessions(ui.sessionsPayload))
	ui.syncDetailHostHeight(false)
	ui.updateShellWidth(false)
}

func (ui *gtkUI) animateFrozenListTransition(fromWidth, toWidth, fromHeight, toHeight int, animate bool, onDone func()) {
	if ui.shell == nil || ui.closingHost == nil {
		return
	}
	debugAnimationLog("animateFrozenListTransition mode=%d fromWidth=%d toWidth=%d fromHeight=%d toHeight=%d animate=%v", ui.listTransitionMode, fromWidth, toWidth, fromHeight, toHeight, animate)
	if fromHeight < 0 {
		fromHeight = 0
	}
	if toHeight < 0 {
		toHeight = 0
	}
	if fromWidth <= 0 {
		fromWidth = ui.shell.Width()
	}
	if fromWidth <= 0 {
		fromWidth = ui.closedShellWidth()
	}
	if toWidth <= 0 {
		toWidth = ui.closedShellWidth()
	}

	ui.stopWidthAnimation()
	ui.stopDetailAnimation()
	ui.cancelPendingDetailOpen()

	ui.widthAnimFrom = fromWidth
	ui.widthAnimTo = toWidth
	ui.widthAnimCurrent = fromWidth
	ui.detailAnimFrom = fromHeight
	ui.detailAnimTo = toHeight

	ui.closingHost.SetVisible(true)
	ui.closingHost.SetOverflowHidden()
	ui.closingHost.ClipSetHeight(fromHeight)
	ui.closingHost.QueueResize()
	ui.closingActive = true

	finish := func() {
		debugAnimationLog("animateFrozenListTransition done mode=%d finalWidth=%d", ui.listTransitionMode, toWidth)
		ui.widthAnimSource = 0
		ui.applyShellWidth(toWidth)
		ui.closingActive = false
		if onDone != nil {
			gtkmini.IdleAdd(onDone)
		}
	}

	if !animate {
		finish()
		return
	}

	start := time.Now()
	duration := scaledDuration(widthAnimationDuration)
	ui.widthAnimSource = gtkmini.TimeoutAdd(16, func() bool {
		progress := float64(time.Since(start)) / float64(duration)
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}
		eased := easeOutCubic(progress)

		width := int(float64(ui.widthAnimFrom) + float64(ui.widthAnimTo-ui.widthAnimFrom)*eased + 0.5)
		height := int(float64(ui.detailAnimFrom) + float64(ui.detailAnimTo-ui.detailAnimFrom)*progress + 0.5)

		ui.applyShellWidth(width)
		ui.closingHost.ClipSetHeight(height)
		ui.closingHost.QueueResize()

		if progress >= 1 {
			finish()
			return false
		}
		return true
	})
}

func (ui *gtkUI) startFrozenListOpen(sessions []payloadSession, animate bool) {
	if ui.closingHost == nil || ui.closingListPage == nil || ui.shell == nil {
		return
	}
	debugAnimationLog("startFrozenListOpen animate=%v sessions=%d", animate, len(sessions))
	ui.listTransitionMode = listTransitionOpening
	ui.rebuildListInto(ui.closingListPage, sessions, false)
	if ui.detailHost != nil {
		ui.detailHost.SetVisible(false)
		ui.detailHost.ClipSetHeight(0)
	}
	ui.closingHost.SetVisible(true)
	ui.closingHost.ClipSetHeight(0)
	ui.closingHost.QueueResize()
	currentWidth := ui.shell.Width()
	if ui.widthAnimSource != 0 {
		currentWidth = ui.widthAnimCurrent
	}
	gtkmini.IdleAdd(func() {
		targetHeight := ui.frozenListHeight()
		targetWidth := ui.frozenListShellWidth()
		ui.animateFrozenListTransition(currentWidth, targetWidth, 0, targetHeight, animate, func() {
			ui.completeFrozenListOpen()
		})
	})
}

func (ui *gtkUI) startFrozenListClose(sessions []payloadSession, fromHeight int, animate bool) {
	if ui.closingHost == nil || ui.closingListPage == nil || ui.shell == nil {
		return
	}
	debugAnimationLog("startFrozenListClose animate=%v sessions=%d fromHeight=%d", animate, len(sessions), fromHeight)
	ui.listTransitionMode = listTransitionClosing
	ui.rebuildListInto(ui.closingListPage, sessions, false)
	ui.resetLiveSlideToList()
	if fromHeight <= 0 {
		fromHeight = ui.frozenListHeight()
	}
	if ui.detailHost != nil {
		ui.detailHost.SetVisible(false)
		ui.detailHost.ClipSetHeight(0)
	}
	currentWidth := ui.shell.Width()
	if ui.widthAnimSource != 0 {
		currentWidth = ui.widthAnimCurrent
	}
	ui.animateFrozenListTransition(currentWidth, ui.closedShellWidth(), fromHeight, 0, animate, func() {
		ui.listTransitionMode = listTransitionNone
		ui.hideFrozenListHost()
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
	if ui.slide == nil {
		return
	}
	debugAnimationLog("setStackView current=%d target=%d", ui.stackView, panelView)

	ui.stopSlideAnimation()
	widthFrom := 0
	widthTo := 0
	animateWidthWithSlide := ui.shell != nil && ui.stackView != panelView
	if animateWidthWithSlide {
		if ui.widthAnimSource != 0 {
			widthFrom = ui.widthAnimCurrent
		} else if ui.shell != nil {
			widthFrom = ui.shell.Width()
		}
		if widthFrom <= 0 {
			widthFrom = ui.measuredWidthForPanelView(ui.stackView)
		}
		ui.stopWidthAnimation()
		if ui.shell != nil && widthFrom > 0 {
			ui.applyShellWidth(widthFrom)
		}
	}

	if panelView == panelViewDetail {
		ui.slide.SlideSetShowingDetail(true)
		if ui.stackView == panelViewDetail {
			ui.listDetailAnimating = false
			ui.slide.SlideSetProgress(1)
			if ui.detailPage != nil {
				ui.detailPage.SetOpacity(1)
			}
		} else {
			widthTo = ui.measuredWidthForPanelView(panelViewDetail)
			ui.slideAnimFrom = 0
			ui.slideAnimTo = 1
			ui.slide.SlideSetProgress(ui.slideAnimFrom)
			if ui.detailPage != nil {
				ui.detailPage.SetOpacity(0)
			}
			ui.listDetailAnimating = animateWidthWithSlide && widthFrom > 0 && widthTo > 0 && widthFrom != widthTo
			ui.widthAnimFrom = widthFrom
			ui.widthAnimTo = widthTo
			ui.widthAnimCurrent = widthFrom
			debugAnimationLog("setStackView detail width from=%d to=%d animate=%v", widthFrom, widthTo, ui.listDetailAnimating)
			start := time.Now()
			duration := scaledDuration(listDetailAnimationDuration)
			ui.slideAnimSource = gtkmini.TimeoutAdd(16, func() bool {
				progress := float64(time.Since(start)) / float64(duration)
				if progress < 0 {
					progress = 0
				}
				if progress > 1 {
					progress = 1
				}
				eased := easeOutCubic(progress)
				value := ui.slideAnimFrom + (ui.slideAnimTo-ui.slideAnimFrom)*eased
				ui.slide.SlideSetProgress(value)
				if ui.detailPage != nil {
					ui.detailPage.SetOpacity(eased)
				}
				if ui.shell != nil && ui.widthAnimFrom > 0 && ui.widthAnimTo > 0 {
					width := int(float64(ui.widthAnimFrom) + float64(ui.widthAnimTo-ui.widthAnimFrom)*eased + 0.5)
					ui.applyShellWidth(width)
				}
				if progress >= 1 {
					ui.slideAnimSource = 0
					ui.listDetailAnimating = false
					if ui.detailPage != nil {
						ui.detailPage.SetOpacity(1)
					}
					if ui.shell != nil && ui.widthAnimTo > 0 {
						ui.applyShellWidth(ui.widthAnimTo)
					}
					if ui.panelView == panelViewDetail {
						ui.syncDetailHostHeight(false)
					}
					return false
				}
				return true
			})
		}
		ui.showingDetail = true
		ui.stackView = panelViewDetail
		return
	}

	ui.slide.SlideSetShowingDetail(false)
	if ui.stackView == panelViewList {
		ui.listDetailAnimating = false
		ui.slide.SlideSetProgress(1)
		if ui.detailPage != nil {
			ui.detailPage.SetOpacity(0)
		}
	} else {
		widthTo = ui.measuredWidthForPanelView(panelViewList)
		ui.slideAnimFrom = 0
		ui.slideAnimTo = 1
		ui.slide.SlideSetProgress(ui.slideAnimFrom)
		if ui.detailPage != nil {
			ui.detailPage.SetOpacity(1)
		}
		ui.listDetailAnimating = animateWidthWithSlide && widthFrom > 0 && widthTo > 0 && widthFrom != widthTo
		ui.widthAnimFrom = widthFrom
		ui.widthAnimTo = widthTo
		ui.widthAnimCurrent = widthFrom
		debugAnimationLog("setStackView list width from=%d to=%d animate=%v", widthFrom, widthTo, ui.listDetailAnimating)
		start := time.Now()
		duration := scaledDuration(listDetailAnimationDuration)
		ui.slideAnimSource = gtkmini.TimeoutAdd(16, func() bool {
			progress := float64(time.Since(start)) / float64(duration)
			if progress < 0 {
				progress = 0
			}
			if progress > 1 {
				progress = 1
			}
			eased := easeOutCubic(progress)
			value := ui.slideAnimFrom + (ui.slideAnimTo-ui.slideAnimFrom)*eased
			ui.slide.SlideSetProgress(value)
			if ui.detailPage != nil {
				ui.detailPage.SetOpacity(1 - eased)
			}
			if ui.shell != nil && ui.widthAnimFrom > 0 && ui.widthAnimTo > 0 {
				width := int(float64(ui.widthAnimFrom) + float64(ui.widthAnimTo-ui.widthAnimFrom)*eased + 0.5)
				ui.applyShellWidth(width)
			}
			if progress >= 1 {
				ui.slideAnimSource = 0
				ui.listDetailAnimating = false
				if ui.detailPage != nil {
					ui.detailPage.SetOpacity(0)
				}
				if ui.shell != nil && ui.widthAnimTo > 0 {
					ui.applyShellWidth(ui.widthAnimTo)
				}
				return false
			}
			return true
		})
	}
	ui.showingDetail = false
	ui.stackView = panelViewList
}

func (ui *gtkUI) openDetail(sessionID string) {
	if sessionID == "" {
		return
	}
	debugAnimationLog("openDetail session=%s", sessionID)
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
	debugAnimationLog("openList")
	ui.cancelHoverClose()
	ui.schedulePanelView(panelViewList)
}

func (ui *gtkUI) closePanel() {
	debugAnimationLog("closePanel")
	ui.cancelHoverClose()
	ui.panelPinned = false
	ui.schedulePanelView(panelViewClosed)
}

func (ui *gtkUI) pinPanel() {
	if ui.panelView == panelViewClosed {
		return
	}
	ui.cancelHoverClose()
	ui.panelPinned = true
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
	if ui.panelPinned {
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

func (ui *gtkUI) buildSessionRow(session payloadSession, interactive bool) *gtkmini.Widget {
	rowShell := gtkmini.NewBox(gtkmini.OrientationVertical, 0)
	rowShell.AddCSSClass("session-row-shell")

	row := gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	row.AddCSSClass("session-row")
	rowShell.Append(row)
	if session.LastUserMessage != "" {
		rowShell.SetTooltipText(session.LastUserMessage)
		row.SetTooltipText(session.LastUserMessage)
	}
	if interactive {
		row.ConnectClick(func() {
			ui.pinPanel()
			ui.openDetail(session.ID)
		})
	}

	dot := gtkmini.NewBox(gtkmini.OrientationHorizontal, 0)
	dot.AddCSSClass("island-status")
	dot.AddCSSClass(statusClass(session.State))
	dot.SetVAlign(gtkmini.AlignCenter)
	row.Append(dot)

	textBox := gtkmini.NewBox(gtkmini.OrientationVertical, 2)
	textBox.SetHexpand(true)

	title := gtkmini.NewLabel(displayName(session))
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
	ui.rebuildListInto(ui.listPage, sessions, true)
}

func (ui *gtkUI) rebuildListInto(container *gtkmini.Widget, sessions []payloadSession, interactive bool) {
	if container == nil {
		return
	}

	container.ClearBoxChildren()

	header := gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	header.AddCSSClass("detail-header")

	title := gtkmini.NewLabel("Sessions")
	title.AddCSSClass("detail-title")
	title.LabelSetXAlign(0)
	header.Append(title)
	container.Append(header)

	for _, session := range sessions {
		container.Append(ui.buildSessionRow(session, interactive))
	}
}

func (ui *gtkUI) rebuildSelectedDetail(sessions []payloadSession) bool {
	if ui.detailPage == nil || ui.selectedSessionID == "" {
		return false
	}

	ui.detailPage.ClearBoxChildren()
	ui.cachedDetailHeight = 0

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
		ui.pinPanel()
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

	if detail.Agent != "" {
		agentRow := buildDetailMetaRow("Agent", detail.Agent, false, nil)
		cardContent.Append(agentRow)
	}

	if detail.AgentName != "" {
		agentNameRow := buildDetailMetaRow("Agent名", detail.AgentName, false, nil)
		cardContent.Append(agentNameRow)
	}

	sessionID := detail.SessionID
	sessionIDRow := buildDetailMetaRow("SessionID", sessionID, true, func(widget *gtkmini.Widget) {
		widget.CopyToClipboard(sessionID)
	})
	cardContent.Append(sessionIDRow)

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

func buildDetailMetaRow(labelText, valueText string, clickable bool, onClick func(widget *gtkmini.Widget)) *gtkmini.Widget {
	rowShell := gtkmini.NewBox(gtkmini.OrientationHorizontal, 0)
	rowShell.AddCSSClass("detail-meta-row-shell")

	row := gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	row.AddCSSClass("detail-meta-row")
	if clickable {
		row.AddCSSClass("detail-meta-row-clickable")
		row.SetTooltipText("Click to copy")
	}
	rowShell.Append(row)

	rowContent := gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	rowContent.AddCSSClass("detail-meta-row-content")
	row.Append(rowContent)

	label := gtkmini.NewLabel(labelText)
	label.AddCSSClass("detail-meta-label")
	label.LabelSetXAlign(0)
	rowContent.Append(label)

	value := gtkmini.NewLabel(valueText)
	value.AddCSSClass("detail-meta-value")
	value.LabelSetXAlign(0)
	value.LabelSetEllipsizeEnd()
	value.SetHexpand(true)
	rowContent.Append(value)

	if clickable {
		hint := gtkmini.NewLabel("Click to copy")
		hint.AddCSSClass("detail-meta-hint")
		hint.LabelSetXAlign(1)
		rowContent.Append(hint)

		var resetSource gtkmini.SourceID

		row.ConnectClick(func() {
			if onClick != nil {
				onClick(row)
			}
			if resetSource != 0 {
				gtkmini.SourceRemove(resetSource)
			}
			hint.LabelSetText("Copied!")
			row.SetTooltipText("Copied!")
			resetSource = gtkmini.TimeoutAdd(1500, func() bool {
				hint.LabelSetText("Click to copy")
				row.SetTooltipText("Click to copy")
				resetSource = 0
				return false
			})
		})
	}

	return rowShell
}

func (ui *gtkUI) rebuildDetail(sessions []payloadSession) {
	if ui.slide == nil {
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
		ui.panelPinned = false
		ui.selectedSessionID = ""
		ui.resetLiveSlideToList()
	}

	ui.rebuildPill(sessions, vm.Pill)
	ui.syncBackdropVisibility()
	debugAnimationLog("rebuildUI panel=%d stackView=%d hasSessions=%v animateWidth=%v closingActive=%v mode=%d wasExpanded=%v pinned=%v shellWidth=%d", ui.panelView, ui.stackView, vm.HasSessions, animateWidth, ui.closingActive, ui.listTransitionMode, ui.wasExpanded, ui.panelPinned, ui.shell.Width())

	if ui.closingActive {
		if ui.panelView == panelViewDetail {
			ui.closingActive = false
			ui.listTransitionMode = listTransitionNone
			ui.hideFrozenListHost()
		} else {
			if ui.panelView != panelViewClosed {
				ui.shell.AddCSSClass("expanded")
			}
			return
		}
	}

	if ui.panelView != panelViewClosed && vm.HasSessions {
		ui.shell.AddCSSClass("expanded")
		if ui.panelView == panelViewDetail {
			ui.shell.AddCSSClass("detail-view")
		} else {
			ui.shell.RemoveCSSClass("detail-view")
		}
		if ui.panelView == panelViewList && !ui.wasExpanded && animateWidth {
			ui.startFrozenListOpen(sessions, true)
			return
		}
		ui.wasExpanded = true
		ui.listTransitionMode = listTransitionNone
		ui.hideFrozenListHost()
		ui.rebuildDetail(sessions)
		ui.syncDetailHostHeight(animateWidth)
		if !ui.listDetailAnimating {
			ui.updateShellWidth(animateWidth)
		}
		return
	}

	ui.shell.RemoveCSSClass("expanded")
	ui.shell.RemoveCSSClass("detail-view")
	if !ui.wasExpanded {
		if ui.detailHost != nil {
			ui.detailHost.SetVisible(false)
			ui.detailHost.ClipSetHeight(0)
		}
		ui.listTransitionMode = listTransitionNone
		ui.hideFrozenListHost()
		closedWidth := ui.closedShellWidth()
		ui.applyShellWidth(closedWidth)
		ui.syncBackdropVisibility()
		return
	}
	ui.wasExpanded = false
	currentHeight := 0
	if ui.detailHost != nil && ui.detailHost.Height() > 0 {
		currentHeight = ui.detailHost.Height()
	} else if ui.cachedListHeight > 0 {
		currentHeight = ui.cachedListHeight
	}
	if currentHeight > 0 && ui.closingListPage != nil {
		ui.startFrozenListClose(sessions, currentHeight, animateWidth)
	}
	ui.syncBackdropVisibility()
}

func (ui *gtkUI) buildShellWidget() *gtkmini.Widget {
	ui.shell = gtkmini.NewShell()

	ui.root = gtkmini.NewBox(gtkmini.OrientationVertical, 0)
	gtkmini.ShellSetChild(ui.shell, ui.root)

	ui.pill = gtkmini.NewBox(gtkmini.OrientationHorizontal, 8)
	ui.pill.AddCSSClass("island-content")
	ui.pill.SetHexpand(true)
	ui.root.Append(ui.pill)

	ui.detailHost = gtkmini.NewClip()
	ui.detailHost.SetHexpand(true)
	ui.detailHost.SetHAlign(gtkmini.AlignFill)
	ui.detailHost.SetOverflowHidden()
	ui.detailHost.ClipSetHeight(0)
	ui.detailHost.SetVisible(false)
	ui.root.Append(ui.detailHost)

	ui.closingHost = gtkmini.NewClip()
	ui.closingHost.SetHexpand(true)
	ui.closingHost.SetHAlign(gtkmini.AlignFill)
	ui.closingHost.SetOverflowHidden()
	ui.closingHost.ClipSetHeight(0)
	ui.closingHost.SetVisible(false)
	ui.root.Append(ui.closingHost)

	ui.slide = gtkmini.NewSlide()
	ui.slide.SetHexpand(true)
	ui.slide.SetHAlign(gtkmini.AlignFill)
	ui.slide.SlideSetShowingDetail(false)
	ui.slide.SlideSetProgress(1)
	gtkmini.ClipSetChild(ui.detailHost, ui.slide)

	ui.listPage = gtkmini.NewBox(gtkmini.OrientationVertical, 2)
	ui.listPage.AddCSSClass("island-detail")
	ui.listPage.AddCSSClass("detail-page")
	ui.listPage.SetHexpand(true)
	ui.listPage.SetHAlign(gtkmini.AlignFill)
	ui.listPage.ConnectClick(func() {
		ui.pinPanel()
	})
	gtkmini.SlideSetListChild(ui.slide, ui.listPage)

	ui.detailPage = gtkmini.NewBox(gtkmini.OrientationVertical, 2)
	ui.detailPage.AddCSSClass("island-detail")
	ui.detailPage.AddCSSClass("detail-page")
	ui.detailPage.SetHexpand(true)
	ui.detailPage.SetHAlign(gtkmini.AlignFill)
	ui.detailPage.ConnectClick(func() {
		ui.pinPanel()
	})
	gtkmini.SlideSetDetailChild(ui.slide, ui.detailPage)
	ui.setStackView(panelViewList)

	ui.closingListPage = gtkmini.NewBox(gtkmini.OrientationVertical, 2)
	ui.closingListPage.AddCSSClass("island-detail")
	ui.closingListPage.AddCSSClass("detail-page")
	ui.closingListPage.SetHexpand(true)
	ui.closingListPage.SetHAlign(gtkmini.AlignFill)
	gtkmini.ClipSetChild(ui.closingHost, ui.closingListPage)

	ui.shell.ConnectHover(func() {
		ui.onHoverEnter()
	}, func() {
		ui.onHoverLeave()
	})

	ui.rebuildUI(ui.sessionsPayload)
	return ui.shell
}

func (ui *gtkUI) buildBackdropWidget() *gtkmini.Widget {
	ui.backdrop = gtkmini.NewBox(gtkmini.OrientationVertical, 0)
	ui.backdrop.AddCSSClass("click-capture")
	ui.backdrop.SetHexpand(true)
	ui.backdrop.SetVexpand(true)
	ui.backdrop.SetHAlign(gtkmini.AlignFill)
	ui.backdrop.SetVAlign(gtkmini.AlignFill)
	ui.backdrop.ConnectClick(func() {
		ui.closePanel()
	})
	return ui.backdrop
}

func (ui *gtkUI) onActivate() {
	ui.backdropWindow = gtkmini.NewApplicationWindow(ui.app)
	ui.window = gtkmini.NewApplicationWindow(ui.app)
	ui.cssProvider = gtkmini.NewCSSProvider()
	if ui.cssData != "" {
		ui.cssProvider.LoadFromString(ui.cssData)
	}

	ui.backdropWindow.SetTitle("way-island-backdrop")
	ui.backdropWindow.SetResizable(false)
	ui.backdropWindow.SetDecorated(false)
	ui.window.SetTitle("way-island")
	ui.window.SetResizable(false)
	ui.window.SetDecorated(false)
	gtkmini.AddProviderForDisplay(ui.backdropWindow.Widget(), ui.cssProvider)
	gtkmini.AddProviderForDisplay(ui.window.Widget(), ui.cssProvider)
	if width, height, ok := gtkmini.DisplayFirstMonitorSize(ui.backdropWindow.Widget()); ok {
		ui.backdropWindow.SetDefaultSize(width, height)
	} else {
		ui.backdropWindow.SetDefaultSize(4096, 4096)
	}

	backdrop := ui.buildBackdropWidget()
	shell := ui.buildShellWidget()
	if ui.shouldQuit {
		ui.app.Quit()
		return
	}

	ui.backdropWindow.Widget().AddCSSClass("way-island-window")
	ui.backdropWindow.Widget().AddCSSClass("click-capture-window")
	ui.backdropWindow.Widget().RemoveCSSClass("background")
	ui.backdropWindow.Widget().RemoveCSSClass("solid-csd")
	ui.backdropWindow.Widget().ConnectClick(func() {
		ui.closePanel()
	})
	ui.backdropWindow.SetChild(backdrop)
	gtkmini.LayerShellConfigureFullscreen(ui.backdropWindow)
	ui.backdropWindow.Present()
	ui.backdropWindow.Widget().SetVisible(false)

	ui.window.Widget().AddCSSClass("way-island-window")
	ui.window.Widget().RemoveCSSClass("background")
	ui.window.Widget().RemoveCSSClass("solid-csd")
	ui.window.SetChild(shell)
	gtkmini.LayerShellConfigureOverlay(ui.window)
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
	ui.cancelPendingDetailOpen()
	ui.stopWidthAnimation()
	ui.stopDetailAnimation()
	ui.stopSlideAnimation()
	ui.widthAnimCurrent = 0
	ui.closingActive = false
	ui.wasExpanded = false
	if ui.panelUpdateSource != 0 {
		gtkmini.SourceRemove(ui.panelUpdateSource)
		ui.panelUpdateSource = 0
	}
	if ui.cssProvider != nil {
		ui.cssProvider.Unref()
		ui.cssProvider = nil
	}
	ui.app.Unref()

	ui.backdropWindow = nil
	ui.window = nil
	ui.backdrop = nil
	ui.shell = nil
	ui.root = nil
	ui.pill = nil
	ui.detailHost = nil
	ui.closingHost = nil
	ui.slide = nil
	ui.listPage = nil
	ui.detailPage = nil
	ui.closingListPage = nil
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
