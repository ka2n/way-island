//go:build gtk4

package main

import (
	"path/filepath"
	"testing"

	"github.com/ka2n/way-island/internal/gtkmini"
)

func TestGTKScenarioBasicPanelFlow(t *testing.T) {
	payload := encodePayloadSessions([]payloadSession{
		{
			ID:     "session-1",
			Name:   "Alpha",
			State:  "tool_running",
			Action: "Bash: go test ./... --very-long-command --with-extra-output --to-force-a-wider-detail-layout-than-the-list-view",
		},
	})

	err := withGTKTestUI(payload, panelViewClosed, "", func(window *gtkmini.Window, widget *gtkmini.Widget, ui *gtkUI) error {
		saveStage := func(name string) {
			t.Helper()
			path := renderTestOutputPath(t, filepath.Join("scenario-basic-flow", name))
			if ok := gtkmini.SaveWidgetPNG(window, widget, path); !ok {
				t.Logf("skip scenario snapshot %s", name)
			}
		}

		closedWidth := ui.shell.Width()
		saveStage("00-closed.png")

		ui.openList()
		gtkmini.PumpEvents(1)
		listEarlyWidth := ui.shell.Width()
		saveStage("01-list-early.png")

		listSettlePumps, err := pumpGTKUntilStable(ui, 240)
		if err != nil {
			t.Fatal(err)
		}
		listWidth := ui.shell.Width()
		saveStage("02-list-settled.png")

		ui.openDetail("session-1")
		gtkmini.PumpEvents(1)
		detailEarlyWidth := ui.shell.Width()
		saveStage("03-detail-early.png")

		gtkmini.PumpEvents(20)
		detailMidWidth := ui.shell.Width()
		saveStage("04-detail-mid.png")

		detailSettlePumps, err := pumpGTKUntilStable(ui, 240)
		if err != nil {
			t.Fatal(err)
		}
		detailWidth := ui.shell.Width()
		saveStage("05-detail-settled.png")

		ui.schedulePanelView(panelViewList)
		gtkmini.PumpEvents(1)
		backEarlyWidth := ui.shell.Width()
		saveStage("06-back-to-list-early.png")

		gtkmini.PumpEvents(20)
		backMidWidth := ui.shell.Width()
		saveStage("07-back-to-list-mid.png")

		backSettlePumps, err := pumpGTKUntilStable(ui, 240)
		if err != nil {
			t.Fatal(err)
		}
		backListWidth := ui.shell.Width()
		saveStage("08-back-to-list-settled.png")

		ui.closePanel()
		closeSettlePumps, err := pumpGTKUntilStable(ui, 240)
		if err != nil {
			t.Fatal(err)
		}
		closedAgainWidth := ui.shell.Width()
		saveStage("09-closed-again.png")

		t.Logf(
			"widths closed=%d list_early=%d list=%d detail_early=%d detail_mid=%d detail=%d back_early=%d back_mid=%d back_list=%d closed_again=%d settle_pumps(list=%d detail=%d back=%d close=%d)",
			closedWidth,
			listEarlyWidth,
			listWidth,
			detailEarlyWidth,
			detailMidWidth,
			detailWidth,
			backEarlyWidth,
			backMidWidth,
			backListWidth,
			closedAgainWidth,
			listSettlePumps,
			detailSettlePumps,
			backSettlePumps,
			closeSettlePumps,
		)

		if listWidth <= closedWidth {
			t.Fatalf("list width did not expand: closed=%d list=%d", closedWidth, listWidth)
		}
		if detailWidth <= listWidth {
			t.Fatalf("detail width did not exceed list width: list=%d detail=%d", listWidth, detailWidth)
		}
		if detailEarlyWidth >= detailWidth {
			t.Fatalf("detail width jumped to final width too early: early=%d final=%d", detailEarlyWidth, detailWidth)
		}
		if detailMidWidth <= detailEarlyWidth || detailMidWidth >= detailWidth {
			t.Fatalf("detail mid width is not between early and final: early=%d mid=%d final=%d", detailEarlyWidth, detailMidWidth, detailWidth)
		}
		if backEarlyWidth <= backListWidth {
			t.Fatalf("return-to-list width jumped to final width too early: early=%d final=%d", backEarlyWidth, backListWidth)
		}
		if backMidWidth >= backEarlyWidth || backMidWidth <= backListWidth {
			t.Fatalf("return-to-list mid width is not between early and final: early=%d mid=%d final=%d", backEarlyWidth, backMidWidth, backListWidth)
		}
		if backListWidth != listWidth {
			t.Fatalf("list width changed after detail round-trip: initial=%d final=%d", listWidth, backListWidth)
		}
		if closedAgainWidth >= backListWidth {
			t.Fatalf("closed width did not shrink: back_list=%d closed_again=%d", backListWidth, closedAgainWidth)
		}

		return nil
	})
	if err != nil {
		t.Skipf("skip GTK scenario test: %v", err)
	}
}

func TestGTKScenarioDetailHeightResyncAfterWidthAnimation(t *testing.T) {
	initialPayload := encodePayloadSessions([]payloadSession{
		{
			ID:     "session-1",
			Name:   "Alpha",
			State:  "tool_running",
			Action: "Bash: SUPERCALIFRAGILISTICEXPIALIDOCIOUS_SUPERCALIFRAGILISTICEXPIALIDOCIOUS_SUPERCALIFRAGILISTICEXPIALIDOCIOUS",
		},
	})
	updatedPayload := encodePayloadSessions([]payloadSession{
		{
			ID:    "session-1",
			Name:  "Alpha",
			State: "tool_running",
			Action: "Bash: this action is intentionally made of many short words so the detail layout " +
				"can wrap across multiple lines after the shell width animation settles to a narrower size",
		},
	})

	err := withGTKTestUI(initialPayload, panelViewDetail, "session-1", func(window *gtkmini.Window, widget *gtkmini.Widget, ui *gtkUI) error {
		initialSettlePumps, err := pumpGTKUntilStable(ui, 240)
		if err != nil {
			t.Fatal(err)
		}
		initialWidth := ui.shell.Width()
		initialHeight := ui.detailHost.Height()

		ui.sessionsPayload = updatedPayload
		ui.rebuildUI(updatedPayload)
		gtkmini.PumpEvents(1)
		updatedEarlyHeight := ui.detailHost.Height()

		updatedSettlePumps, err := pumpGTKUntilStable(ui, 240)
		if err != nil {
			t.Fatal(err)
		}
		updatedWidth := ui.shell.Width()
		updatedHeight := ui.detailHost.Height()

		t.Logf(
			"detail resync widths initial=%d updated=%d heights initial=%d updated_early=%d updated=%d settle_pumps(initial=%d updated=%d)",
			initialWidth,
			updatedWidth,
			initialHeight,
			updatedEarlyHeight,
			updatedHeight,
			initialSettlePumps,
			updatedSettlePumps,
		)

		if updatedWidth >= initialWidth {
			t.Fatalf("expected updated detail width to shrink: initial=%d updated=%d", initialWidth, updatedWidth)
		}
		if updatedHeight <= initialHeight {
			t.Fatalf("expected updated detail height to grow after width shrink: initial=%d updated=%d", initialHeight, updatedHeight)
		}
		if updatedHeight <= updatedEarlyHeight {
			t.Fatalf("expected detail height to resync after animation: early=%d updated=%d", updatedEarlyHeight, updatedHeight)
		}

		return nil
	})
	if err != nil {
		t.Skipf("skip GTK scenario test: %v", err)
	}
}
