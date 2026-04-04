package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
)

const (
	renderWidth           = 360
	renderCollapsedHeight = 56
	renderListRowHeight   = 44
	renderListHeaderGap   = 28
	renderDetailHeight    = 190
)

var (
	renderBackground      = color.RGBA{12, 15, 22, 240}
	renderPanelBackground = color.RGBA{15, 23, 42, 184}
	renderBorder          = color.RGBA{110, 125, 148, 96}
	renderHover           = color.RGBA{148, 163, 184, 26}
	renderTextPrimary     = color.RGBA{248, 250, 252, 255}
	renderTextSecondary   = color.RGBA{148, 163, 184, 255}
	renderTextMuted       = color.RGBA{203, 213, 225, 255}
	renderWorking         = color.RGBA{74, 222, 128, 255}
	renderToolRunning     = color.RGBA{96, 165, 250, 255}
	renderWaiting         = color.RGBA{251, 191, 36, 255}
	renderIdle            = color.RGBA{148, 163, 184, 255}
)

func renderOverlayViewModel(vm overlayViewModel) *image.RGBA {
	height := renderCollapsedHeight
	if vm.Expanded {
		if vm.StackView == panelViewDetail {
			height += renderDetailHeight
		} else {
			height += renderListHeaderGap + len(vm.ListRows)*renderListRowHeight + 12
		}
	}

	img := image.NewRGBA(image.Rect(0, 0, renderWidth, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{0, 0, 0, 0}}, image.Point{}, draw.Src)

	drawFilledRect(img, image.Rect(0, 0, renderWidth, height), renderBackground)
	drawFilledRect(img, image.Rect(0, 0, renderWidth, renderCollapsedHeight), renderBackground)
	drawBorder(img, image.Rect(0, 0, renderWidth, height), renderBorder)

	drawPill(img, vm.Pill)

	if !vm.Expanded {
		return img
	}

	contentTop := renderCollapsedHeight
	drawHorizontalLine(img, contentTop, renderBorder)
	if vm.StackView == panelViewDetail && vm.Detail != nil {
		drawDetail(img, contentTop+1, *vm.Detail)
		return img
	}

	drawList(img, contentTop+1, vm.ListTitle, vm.ListRows)
	return img
}

func writeOverlayPNG(path string, vm overlayViewModel) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, renderOverlayViewModel(vm))
}

func drawPill(img *image.RGBA, pill pillViewModel) {
	drawText(img, 16, 22, pill.Title, renderTextPrimary)
	drawFilledRect(img, image.Rect(16, 30, 24, 38), renderStatusColor(pill.StateClass))
	if pill.BadgeCount > 1 {
		drawFilledRect(img, image.Rect(renderWidth-34, 14, renderWidth-16, 30), renderHover)
		drawText(img, renderWidth-29, 25, itoa(pill.BadgeCount), renderTextSecondary)
	}
}

func drawList(img *image.RGBA, top int, title string, rows []sessionRowViewModel) {
	drawText(img, 16, top+18, title, renderTextMuted)
	y := top + renderListHeaderGap
	for _, row := range rows {
		drawFilledRect(img, image.Rect(12, y, renderWidth-12, y+36), renderHover)
		drawFilledRect(img, image.Rect(24, y+14, 32, y+22), renderStatusColor(row.StateClass))
		drawText(img, 42, y+16, row.Title, renderTextPrimary)
		drawText(img, 42, y+30, row.DetailText, renderTextSecondary)
		y += renderListRowHeight
	}
}

func drawDetail(img *image.RGBA, top int, detail detailViewModel) {
	drawFilledRect(img, image.Rect(16, top+8, 40, top+32), renderHover)
	drawText(img, 48, top+24, "Session detail", renderTextMuted)

	cardTop := top + 44
	cardRect := image.Rect(16, cardTop, renderWidth-16, cardTop+132)
	drawFilledRect(img, cardRect, renderPanelBackground)
	drawBorder(img, cardRect, renderBorder)

	drawText(img, 30, cardTop+24, detail.Title, renderTextPrimary)
	drawFilledRect(img, image.Rect(30, cardTop+38, 38, cardTop+46), renderStatusColor(detail.StateClass))
	drawText(img, 46, cardTop+45, detail.StatusLabel, renderTextMuted)
	drawText(img, 30, cardTop+70, detail.BodyText, renderTextSecondary)
	drawFilledRect(img, image.Rect(30, cardTop+90, 130, cardTop+112), color.RGBA{226, 232, 240, 255})
	drawText(img, 40, cardTop+105, "Open session", color.RGBA{15, 23, 42, 255})
}

func drawFilledRect(img *image.RGBA, rect image.Rectangle, fill color.Color) {
	draw.Draw(img, rect, &image.Uniform{C: fill}, image.Point{}, draw.Src)
}

func drawBorder(img *image.RGBA, rect image.Rectangle, stroke color.Color) {
	drawHorizontalLineRect(img, rect.Min.Y, rect.Min.X, rect.Max.X, stroke)
	drawHorizontalLineRect(img, rect.Max.Y-1, rect.Min.X, rect.Max.X, stroke)
	drawVerticalLine(img, rect.Min.X, rect.Min.Y, rect.Max.Y, stroke)
	drawVerticalLine(img, rect.Max.X-1, rect.Min.Y, rect.Max.Y, stroke)
}

func drawHorizontalLine(img *image.RGBA, y int, stroke color.Color) {
	drawHorizontalLineRect(img, y, 0, img.Bounds().Dx(), stroke)
}

func drawHorizontalLineRect(img *image.RGBA, y, x0, x1 int, stroke color.Color) {
	draw.Draw(img, image.Rect(x0, y, x1, y+1), &image.Uniform{C: stroke}, image.Point{}, draw.Src)
}

func drawVerticalLine(img *image.RGBA, x, y0, y1 int, stroke color.Color) {
	draw.Draw(img, image.Rect(x, y0, x+1, y1), &image.Uniform{C: stroke}, image.Point{}, draw.Src)
}

func drawText(img *image.RGBA, x, y int, text string, fill color.Color) {
	cursor := x
	top := y - 10
	for _, r := range text {
		if r == ' ' {
			cursor += 4
			continue
		}
		draw.Draw(img, image.Rect(cursor, top, cursor+5, top+8), &image.Uniform{C: fill}, image.Point{}, draw.Src)
		cursor += 7
	}
}

func renderStatusColor(stateClass string) color.Color {
	switch stateClass {
	case "working":
		return renderWorking
	case "tool-running":
		return renderToolRunning
	case "waiting":
		return renderWaiting
	default:
		return renderIdle
	}
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}

	var digits [20]byte
	pos := len(digits)
	for v > 0 {
		pos--
		digits[pos] = byte('0' + (v % 10))
		v /= 10
	}
	return string(digits[pos:])
}
