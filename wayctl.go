package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	wlDisplayID = 1
	wlSeatIF    = "wl_seat"

	zwlrForeignToplevelManagerV1IF = "zwlr_foreign_toplevel_manager_v1"

	wlDisplaySyncOpcode        = 0
	wlDisplayGetRegistryOpcode = 1

	wlRegistryBindOpcode        = 0
	wlRegistryGlobalEventOpcode = 0
	wlCallbackDoneEventOpcode   = 0
	wlSeatVersion               = 1
	zwlrManagerVersion          = 1
	zwlrManagerStopOpcode       = 0
	zwlrManagerToplevelEvent    = 0
	zwlrManagerFinishedEvent    = 1
	zwlrHandleTitleEvent        = 0
	zwlrHandleAppIDEvent        = 1
	zwlrHandleStateEvent        = 4
	zwlrHandleDoneEvent         = 5
	zwlrHandleClosedEvent       = 6
	zwlrHandleActivateOpcode    = 4
	zwlrHandleDestroyOpcode     = 6

	waylandReadTimeout = 2 * time.Second
)

var (
	errWaylandUnsupported = errors.New("wayland foreign toplevel management unsupported")
	errWaylandNoSeat      = errors.New("wayland seat unavailable")
)

func focusTerminalWindow(appID string, title string) error {
	log.Printf("wayland focus start app_id=%s title=%q", appID, title)
	client, err := newWaylandFocusClientFromEnv()
	if err != nil {
		log.Printf("wayland focus error app_id=%s title=%q err=%v", appID, title, err)
		return err
	}
	defer client.Close()

	if err := client.Focus(appID, title); err != nil {
		log.Printf("wayland focus error app_id=%s title=%q err=%v", appID, title, err)
		return err
	}
	log.Printf("wayland focus ok app_id=%s title=%q", appID, title)
	return nil
}

type waylandFocusClient struct {
	conn           net.Conn
	nextObjectID   uint32
	readBuf        []byte
	handlers       map[uint32]waylandMessageHandler
	registryID     uint32
	managerID      uint32
	seatID         uint32
	managerName    uint32
	managerVersion uint32
	seatName       uint32
	seatVersion    uint32
	toplevels      map[uint32]*waylandToplevel
}

type waylandToplevel struct {
	id          uint32
	title       string
	appID       string
	initialized bool
	closed      bool
}

type waylandMessageHandler func(opcode uint16, payload []byte) error

func newWaylandFocusClientFromEnv() (*waylandFocusClient, error) {
	socketPath, err := waylandSocketPath()
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTimeout("unix", socketPath, waylandReadTimeout)
	if err != nil {
		return nil, fmt.Errorf("connect wayland display %q: %w", socketPath, err)
	}
	if err := conn.SetDeadline(time.Now().Add(waylandReadTimeout)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set wayland deadline: %w", err)
	}

	client := &waylandFocusClient{
		conn:         conn,
		nextObjectID: wlDisplayID + 1,
		handlers:     map[uint32]waylandMessageHandler{},
		toplevels:    map[uint32]*waylandToplevel{},
	}
	client.handlers[wlDisplayID] = client.handleDisplayEvent

	if err := client.initialize(); err != nil {
		conn.Close()
		return nil, err
	}

	return client, nil
}

func waylandSocketPath() (string, error) {
	display := os.Getenv("WAYLAND_DISPLAY")
	if strings.TrimSpace(display) == "" {
		display = "wayland-0"
	}
	if filepath.IsAbs(display) {
		return display, nil
	}

	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if strings.TrimSpace(runtimeDir) == "" {
		return "", errors.New("XDG_RUNTIME_DIR is not set")
	}
	return filepath.Join(runtimeDir, display), nil
}

func (c *waylandFocusClient) Close() error {
	var firstErr error

	if c.managerID != 0 {
		if err := c.sendRequest(c.managerID, zwlrManagerStopOpcode, nil); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for id := range c.toplevels {
		if err := c.sendRequest(id, zwlrHandleDestroyOpcode, nil); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (c *waylandFocusClient) initialize() error {
	c.registryID = c.newObjectID()
	c.handlers[c.registryID] = c.handleRegistryEvent
	if err := c.sendRequest(wlDisplayID, wlDisplayGetRegistryOpcode, encodeNewID(c.registryID)); err != nil {
		return err
	}
	if err := c.roundTrip(); err != nil {
		return err
	}

	if c.managerName == 0 {
		return errWaylandUnsupported
	}
	if c.seatName == 0 {
		return errWaylandNoSeat
	}

	c.managerID = c.newObjectID()
	c.handlers[c.managerID] = c.handleManagerEvent
	if err := c.bindRegistryObject(c.managerName, zwlrForeignToplevelManagerV1IF, minUint32(c.managerVersion, zwlrManagerVersion), c.managerID); err != nil {
		return err
	}

	c.seatID = c.newObjectID()
	if err := c.bindRegistryObject(c.seatName, wlSeatIF, minUint32(c.seatVersion, wlSeatVersion), c.seatID); err != nil {
		return err
	}

	return c.roundTrip()
}

func (c *waylandFocusClient) Focus(appID string, title string) error {
	target := c.findToplevel(appID, title)
	if target == nil {
		return fmt.Errorf("wayland toplevel not found for app_id=%q title=%q", appID, title)
	}
	log.Printf("wayland focus target app_id=%s requested_title=%q matched_title=%q toplevel_id=%d seat_id=%d", appID, title, target.title, target.id, c.seatID)

	if err := c.sendRequest(target.id, zwlrHandleActivateOpcode, encodeObject(c.seatID)); err != nil {
		return err
	}

	return c.roundTrip()
}

func (c *waylandFocusClient) findToplevel(appID string, title string) *waylandToplevel {
	candidates := make([]*waylandToplevel, 0, len(c.toplevels))
	for _, toplevel := range c.toplevels {
		if toplevel.closed || !toplevel.initialized {
			continue
		}
		if toplevel.appID != appID {
			continue
		}
		candidates = append(candidates, toplevel)
	}
	if len(candidates) == 0 {
		return nil
	}

	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle != "" {
		// Exact title matches keep the old behavior for users who already align
		// tmux window names with terminal titles.
		for _, candidate := range candidates {
			if candidate.title == trimmedTitle {
				return candidate
			}
		}

		var containsMatches []*waylandToplevel
		for _, candidate := range candidates {
			if strings.Contains(candidate.title, trimmedTitle) {
				containsMatches = append(containsMatches, candidate)
			}
		}
		// A unique partial match is usually enough when the terminal title includes
		// extra prefixes such as the tmux session name.
		if len(containsMatches) == 1 {
			return containsMatches[0]
		}
	}

	// When there is only one terminal candidate, prefer focusing something
	// plausible over failing the request outright.
	if len(candidates) == 1 {
		return candidates[0]
	}

	return bestToplevelCandidate(candidates, trimmedTitle)
}

func bestToplevelCandidate(candidates []*waylandToplevel, title string) *waylandToplevel {
	// Keep fallback selection deterministic so repeated focus attempts do not hop
	// between terminals when multiple candidates score the same.
	var best *waylandToplevel
	bestScore := 0
	for _, candidate := range candidates {
		score := scoreToplevelCandidate(candidate.title, title)
		if score <= 0 {
			continue
		}
		if score < bestScore {
			continue
		}
		if score == bestScore && best != nil && candidate.title >= best.title {
			continue
		}
		best = candidate
		bestScore = score
	}
	return best
}

func scoreToplevelCandidate(candidateTitle, requestedTitle string) int {
	if requestedTitle == "" {
		return 0
	}

	score := 0
	if strings.Contains(candidateTitle, requestedTitle) {
		score += 10
	}

	parts := splitTitleParts(requestedTitle)
	if len(parts) == 0 {
		return score
	}
	for _, part := range parts {
		if strings.Contains(candidateTitle, part) {
			score++
		}
	}
	return score
}

func splitTitleParts(title string) []string {
	fields := strings.FieldsFunc(title, func(r rune) bool {
		switch r {
		case ':', '/', '-', '|':
			return true
		default:
			return false
		}
	})

	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		parts = append(parts, field)
	}
	return parts
}

func (c *waylandFocusClient) bindRegistryObject(name uint32, iface string, version uint32, objectID uint32) error {
	payload := make([]byte, 0, 64)
	payload = append(payload, encodeUint32(name)...)
	payload = append(payload, encodeString(iface)...)
	payload = append(payload, encodeUint32(version)...)
	payload = append(payload, encodeNewID(objectID)...)
	return c.sendRequest(c.registryID, wlRegistryBindOpcode, payload)
}

func (c *waylandFocusClient) roundTrip() error {
	callbackID := c.newObjectID()
	done := false
	c.handlers[callbackID] = func(opcode uint16, payload []byte) error {
		if opcode != wlCallbackDoneEventOpcode {
			return nil
		}
		delete(c.handlers, callbackID)
		done = true
		return nil
	}

	if err := c.sendRequest(wlDisplayID, wlDisplaySyncOpcode, encodeNewID(callbackID)); err != nil {
		return err
	}

	return c.dispatchUntil(func() bool { return done })
}

func (c *waylandFocusClient) dispatchUntil(done func() bool) error {
	for !done() {
		if err := c.dispatchNextMessage(); err != nil {
			return err
		}
	}
	return nil
}

func (c *waylandFocusClient) dispatchNextMessage() error {
	for len(c.readBuf) < 8 {
		if err := c.readIntoBuffer(); err != nil {
			return err
		}
	}

	sizeAndOpcode := binary.LittleEndian.Uint32(c.readBuf[4:8])
	size := int(sizeAndOpcode >> 16)
	if size < 8 {
		return fmt.Errorf("invalid wayland message size %d", size)
	}

	for len(c.readBuf) < size {
		if err := c.readIntoBuffer(); err != nil {
			return err
		}
	}

	objectID := binary.LittleEndian.Uint32(c.readBuf[:4])
	opcode := uint16(sizeAndOpcode & 0xffff)
	payload := append([]byte(nil), c.readBuf[8:size]...)
	c.readBuf = c.readBuf[size:]

	handler := c.handlers[objectID]
	if handler == nil {
		return nil
	}
	return handler(opcode, payload)
}

func (c *waylandFocusClient) readIntoBuffer() error {
	if err := c.conn.SetDeadline(time.Now().Add(waylandReadTimeout)); err != nil {
		return err
	}

	tmp := make([]byte, 4096)
	n, err := c.conn.Read(tmp)
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return fmt.Errorf("timed out waiting for wayland event: %w", err)
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return fmt.Errorf("timed out waiting for wayland event: %w", err)
		}
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("wayland connection closed: %w", err)
		}
		return err
	}
	if n == 0 {
		return io.EOF
	}
	c.readBuf = append(c.readBuf, tmp[:n]...)
	return nil
}

func (c *waylandFocusClient) sendRequest(objectID uint32, opcode uint16, payload []byte) error {
	msg := make([]byte, 8+len(payload))
	binary.LittleEndian.PutUint32(msg[:4], objectID)
	binary.LittleEndian.PutUint32(msg[4:8], uint32(len(msg))<<16|uint32(opcode))
	copy(msg[8:], payload)

	for written := 0; written < len(msg); {
		n, err := c.conn.Write(msg[written:])
		if err != nil {
			return err
		}
		written += n
	}
	return nil
}

func (c *waylandFocusClient) newObjectID() uint32 {
	id := c.nextObjectID
	c.nextObjectID++
	return id
}

func (c *waylandFocusClient) handleDisplayEvent(opcode uint16, payload []byte) error {
	return nil
}

func (c *waylandFocusClient) handleRegistryEvent(opcode uint16, payload []byte) error {
	if opcode != wlRegistryGlobalEventOpcode {
		return nil
	}

	dec := newWaylandDecoder(payload)
	name, err := dec.uint32()
	if err != nil {
		return err
	}
	iface, err := dec.string()
	if err != nil {
		return err
	}
	version, err := dec.uint32()
	if err != nil {
		return err
	}

	switch iface {
	case zwlrForeignToplevelManagerV1IF:
		c.managerName = name
		c.managerVersion = version
	case wlSeatIF:
		c.seatName = name
		c.seatVersion = version
	}
	return nil
}

func (c *waylandFocusClient) handleManagerEvent(opcode uint16, payload []byte) error {
	switch opcode {
	case zwlrManagerToplevelEvent:
		dec := newWaylandDecoder(payload)
		handleID, err := dec.newID()
		if err != nil {
			return err
		}
		toplevel := &waylandToplevel{id: handleID}
		c.toplevels[handleID] = toplevel
		c.handlers[handleID] = func(opcode uint16, payload []byte) error {
			return c.handleToplevelEvent(toplevel, opcode, payload)
		}
	case zwlrManagerFinishedEvent:
		return errWaylandUnsupported
	}
	return nil
}

func (c *waylandFocusClient) handleToplevelEvent(toplevel *waylandToplevel, opcode uint16, payload []byte) error {
	switch opcode {
	case zwlrHandleTitleEvent:
		dec := newWaylandDecoder(payload)
		title, err := dec.string()
		if err != nil {
			return err
		}
		toplevel.title = title
	case zwlrHandleAppIDEvent:
		dec := newWaylandDecoder(payload)
		appID, err := dec.string()
		if err != nil {
			return err
		}
		toplevel.appID = appID
	case zwlrHandleStateEvent:
		dec := newWaylandDecoder(payload)
		if _, err := dec.array(); err != nil {
			return err
		}
	case zwlrHandleDoneEvent:
		toplevel.initialized = true
	case zwlrHandleClosedEvent:
		toplevel.closed = true
		delete(c.handlers, toplevel.id)
	}
	return nil
}

type waylandDecoder struct {
	data []byte
	off  int
}

func newWaylandDecoder(data []byte) *waylandDecoder {
	return &waylandDecoder{data: data}
}

func (d *waylandDecoder) uint32() (uint32, error) {
	if len(d.data[d.off:]) < 4 {
		return 0, io.ErrUnexpectedEOF
	}
	value := binary.LittleEndian.Uint32(d.data[d.off : d.off+4])
	d.off += 4
	return value, nil
}

func (d *waylandDecoder) newID() (uint32, error) {
	return d.uint32()
}

func (d *waylandDecoder) string() (string, error) {
	length, err := d.uint32()
	if err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}

	size := int(length)
	padded := align4(size)
	if len(d.data[d.off:]) < padded {
		return "", io.ErrUnexpectedEOF
	}

	raw := d.data[d.off : d.off+size]
	d.off += padded
	if size > 0 && raw[size-1] == 0 {
		raw = raw[:size-1]
	}
	return string(raw), nil
}

func (d *waylandDecoder) array() ([]byte, error) {
	length, err := d.uint32()
	if err != nil {
		return nil, err
	}

	size := int(length)
	padded := align4(size)
	if len(d.data[d.off:]) < padded {
		return nil, io.ErrUnexpectedEOF
	}
	raw := append([]byte(nil), d.data[d.off:d.off+size]...)
	d.off += padded
	return raw, nil
}

func encodeUint32(value uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, value)
	return buf
}

func encodeObject(value uint32) []byte {
	return encodeUint32(value)
}

func encodeNewID(value uint32) []byte {
	return encodeUint32(value)
}

func encodeString(value string) []byte {
	if value == "" {
		return encodeUint32(0)
	}

	length := len(value) + 1
	buf := make([]byte, 4+align4(length))
	binary.LittleEndian.PutUint32(buf[:4], uint32(length))
	copy(buf[4:], value)
	return buf
}

func align4(size int) int {
	return (size + 3) &^ 3
}

func minUint32(a uint32, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
