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
