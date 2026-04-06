package main

const shellExpandedWidth = 340

func clampShellExpandedWidth(width int) int {
	if width <= 0 {
		return shellExpandedWidth
	}
	if width > shellExpandedWidth {
		return shellExpandedWidth
	}
	return width
}
