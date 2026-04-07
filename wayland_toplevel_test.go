package main

import "testing"

func TestWaylandDecoderString(t *testing.T) {
	dec := newWaylandDecoder(encodeString("logs"))

	value, err := dec.string()
	if err != nil {
		t.Fatalf("string: %v", err)
	}
	if value != "logs" {
		t.Fatalf("value = %q, want %q", value, "logs")
	}
}

func TestWaylandFocusClientFindToplevel(t *testing.T) {
	client := &waylandFocusClient{
		toplevels: map[uint32]*waylandToplevel{
			7: {id: 7, appID: "foot", title: "shell", initialized: true},
			8: {id: 8, appID: "Alacritty", title: "main:logs", initialized: true},
			9: {id: 9, appID: "Alacritty", title: "logs", initialized: false},
		},
	}

	match := client.findToplevel("Alacritty", "main:logs")
	if match == nil {
		t.Fatal("findToplevel returned nil")
	}
	if match.id != 8 {
		t.Fatalf("match.id = %d, want %d", match.id, 8)
	}
}

func TestWaylandFocusClientFindToplevelByContainedWindowName(t *testing.T) {
	client := &waylandFocusClient{
		toplevels: map[uint32]*waylandToplevel{
			8: {id: 8, appID: "Alacritty", title: "main:logs", initialized: true},
			9: {id: 9, appID: "Alacritty", title: "main:shell", initialized: true},
		},
	}

	match := client.findToplevel("Alacritty", "logs")
	if match == nil {
		t.Fatal("findToplevel returned nil")
	}
	if match.id != 8 {
		t.Fatalf("match.id = %d, want %d", match.id, 8)
	}
}

func TestWaylandFocusClientFindToplevelReturnsNilForTitleMismatch(t *testing.T) {
	client := &waylandFocusClient{
		toplevels: map[uint32]*waylandToplevel{
			8: {id: 8, appID: "Alacritty", title: "random-title", initialized: true},
		},
	}

	match := client.findToplevel("Alacritty", "logs")
	if match != nil {
		t.Fatalf("findToplevel returned %v, want nil", match)
	}
}

func TestWaylandFocusClientFindToplevelReturnsNilWhenNoTitleMatch(t *testing.T) {
	client := &waylandFocusClient{
		toplevels: map[uint32]*waylandToplevel{
			8: {id: 8, appID: "Alacritty", title: "beta", initialized: true},
			9: {id: 9, appID: "Alacritty", title: "alpha", initialized: true},
		},
	}

	match := client.findToplevel("Alacritty", "missing")
	if match != nil {
		t.Fatalf("findToplevel returned %v, want nil", match)
	}
}
