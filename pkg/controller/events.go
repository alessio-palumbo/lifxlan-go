package controller

import (
	"context"
	"sync"

	"github.com/alessio-palumbo/lifxlan-go/pkg/device"
)

const defaultSubscriptionBufferSize = 64

// DeviceEventType describes a change to the controller's cached device set.
type DeviceEventType uint8

const (
	// DeviceEventAdded indicates that a device has become available. Existing
	// devices are also delivered this way when a subscription starts.
	DeviceEventAdded DeviceEventType = iota + 1
	// DeviceEventUpdated indicates that observed device state changed.
	DeviceEventUpdated
	// DeviceEventRemoved indicates that a device is no longer available.
	DeviceEventRemoved
	// DeviceEventResyncRequired indicates that the subscriber could not keep up
	// and should replace its local view with Controller.GetDevices.
	DeviceEventResyncRequired
	// DeviceEventSnapshotComplete marks the end of a subscription's initial
	// inventory. Its revision is the baseline before live events begin.
	DeviceEventSnapshotComplete
)

// String returns the event type's API representation.
func (t DeviceEventType) String() string {
	switch t {
	case DeviceEventAdded:
		return "added"
	case DeviceEventUpdated:
		return "updated"
	case DeviceEventRemoved:
		return "removed"
	case DeviceEventResyncRequired:
		return "resync_required"
	case DeviceEventSnapshotComplete:
		return "snapshot_complete"
	default:
		return ""
	}
}

// DeviceChange identifies categories changed by a DeviceEventUpdated event.
type DeviceChange uint64

const (
	DeviceChangeLabel DeviceChange = 1 << iota
	DeviceChangeProduct
	DeviceChangeFirmware
	DeviceChangeLocation
	DeviceChangeGroup
	DeviceChangeWiFi
	DeviceChangeLight
	DeviceChangeMatrix
	DeviceChangeMultizone
	DeviceChangeButtons
	DeviceChangeButtonConfig
	DeviceChangeRelays
	DeviceChangeEffect
)

// Has reports whether changes includes every bit in change.
func (changes DeviceChange) Has(change DeviceChange) bool {
	return changes&change == change
}

// DeviceEvent describes a transition in the controller's observed device
// state. Device is an independent snapshot and is empty for resync and
// snapshot-complete events. Initial device events have revision zero; a
// snapshot-complete event carries the baseline revision for subsequent live
// transitions.
type DeviceEvent struct {
	Type     DeviceEventType
	Device   device.Device
	Changes  DeviceChange
	Revision uint64
	Initial  bool
}

type subscriptionConfig struct {
	bufferSize int
}

// SubscriptionOption configures a device-event subscription.
type SubscriptionOption func(*subscriptionConfig)

// WithSubscriptionBufferSize sets the number of pending events retained for a
// subscriber. Values smaller than one are treated as one.
func WithSubscriptionBufferSize(size int) SubscriptionOption {
	return func(cfg *subscriptionConfig) {
		cfg.bufferSize = max(1, size)
	}
}

type deviceSubscription struct {
	events chan DeviceEvent
	wake   chan struct{}
	done   chan struct{}

	stopOnce sync.Once
	mu       sync.Mutex
	queue    []DeviceEvent
	limit    int
	overflow bool
	preface  int
}

func newDeviceSubscription(limit int) *deviceSubscription {
	return &deviceSubscription{
		events: make(chan DeviceEvent),
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
		limit:  limit,
	}
}

func (s *deviceSubscription) enqueue(event DeviceEvent, cloneDevice bool) {
	select {
	case <-s.done:
		return
	default:
	}

	s.mu.Lock()
	if s.overflow {
		s.queue[len(s.queue)-1].Revision = event.Revision
		s.mu.Unlock()
		return
	}
	if len(s.queue) >= s.limit {
		s.limit -= s.preface
		s.preface = 0
		clear(s.queue)
		s.queue = append(s.queue[:0], DeviceEvent{
			Type:     DeviceEventResyncRequired,
			Revision: event.Revision,
		})
		s.overflow = true
	} else {
		if cloneDevice {
			event.Device = event.Device.Clone()
		}
		s.queue = append(s.queue, event)
	}
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *deviceSubscription) initialize(devices []device.Device, revision uint64) {
	select {
	case <-s.done:
		return
	default:
	}

	s.mu.Lock()
	initialCount := len(devices) + 1
	s.limit += initialCount
	s.preface += initialCount
	initial := make([]DeviceEvent, 0, initialCount+len(s.queue))
	for _, d := range devices {
		initial = append(initial, DeviceEvent{
			Type:    DeviceEventAdded,
			Device:  d,
			Initial: true,
		})
	}
	initial = append(initial, DeviceEvent{
		Type:     DeviceEventSnapshotComplete,
		Revision: revision,
	})
	s.queue = append(initial, s.queue...)
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *deviceSubscription) next() (DeviceEvent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.queue) == 0 {
		return DeviceEvent{}, false
	}
	event := s.queue[0]
	copy(s.queue, s.queue[1:])
	s.queue[len(s.queue)-1] = DeviceEvent{}
	s.queue = s.queue[:len(s.queue)-1]
	if event.Initial || event.Type == DeviceEventSnapshotComplete {
		s.preface--
		s.limit--
	}
	if event.Type == DeviceEventResyncRequired {
		s.overflow = false
	}
	return event, true
}

func (s *deviceSubscription) stop() {
	s.stopOnce.Do(func() { close(s.done) })
}

func (s *deviceSubscription) run(ctx context.Context, unregister func()) {
	defer unregister()
	defer close(s.events)

	for {
		if event, ok := s.next(); ok {
			select {
			case s.events <- event:
			case <-ctx.Done():
				return
			case <-s.done:
				return
			}
			continue
		}

		select {
		case <-s.wake:
		case <-ctx.Done():
			return
		case <-s.done:
			return
		}
	}
}

// SubscribeDevices returns a stream of observed device changes. It first emits
// the current devices as initial DeviceEventAdded events with revision zero,
// followed by DeviceEventSnapshotComplete carrying the live revision baseline.
// Live events have revisions greater than that baseline. DeviceEventAdded means
// a session exists, not that its capability-specific state is fully populated.
// The channel closes when ctx is canceled or the Controller closes.
func (c *Controller) SubscribeDevices(ctx context.Context, opts ...SubscriptionOption) <-chan DeviceEvent {
	cfg := subscriptionConfig{bufferSize: defaultSubscriptionBufferSize}
	for _, opt := range opts {
		opt(&cfg)
	}

	subscription := newDeviceSubscription(cfg.bufferSize)

	// Exclude device-set changes and update publication until the initial
	// inventory and its revision boundary have been queued.
	c.mu.Lock()
	c.subscriptionsMu.Lock()
	if c.subscriptionsClosed {
		c.subscriptionsMu.Unlock()
		c.mu.Unlock()
		close(subscription.events)
		return subscription.events
	}
	c.nextSubscriptionID++
	id := c.nextSubscriptionID
	c.subscriptions[id] = subscription
	revision := c.eventRevision
	c.subscriptionsMu.Unlock()

	devices := make([]device.Device, 0, len(c.sessions))
	for _, session := range c.sessions {
		devices = append(devices, session.deviceSnapshot())
	}
	device.SortDevices(devices)
	subscription.initialize(devices, revision)
	c.mu.Unlock()

	go subscription.run(ctx, func() { c.removeSubscription(id, subscription) })
	return subscription.events
}

func (c *Controller) hasSubscriptions() bool {
	c.subscriptionsMu.Lock()
	defer c.subscriptionsMu.Unlock()
	return !c.subscriptionsClosed && len(c.subscriptions) > 0
}

func (c *Controller) publishDeviceEvent(event DeviceEvent) {
	c.subscriptionsMu.Lock()
	if c.subscriptionsClosed {
		c.subscriptionsMu.Unlock()
		return
	}
	c.eventRevision++
	event.Revision = c.eventRevision
	subscriptions := make([]*deviceSubscription, 0, len(c.subscriptions))
	for _, subscription := range c.subscriptions {
		subscriptions = append(subscriptions, subscription)
	}
	c.subscriptionsMu.Unlock()

	for i, subscription := range subscriptions {
		subscription.enqueue(event, i > 0)
	}
}

func (c *Controller) removeSubscription(id uint64, subscription *deviceSubscription) {
	c.subscriptionsMu.Lock()
	if c.subscriptions[id] == subscription {
		delete(c.subscriptions, id)
	}
	c.subscriptionsMu.Unlock()
}

func (c *Controller) closeSubscriptions() {
	c.subscriptionsMu.Lock()
	if c.subscriptionsClosed {
		c.subscriptionsMu.Unlock()
		return
	}
	c.subscriptionsClosed = true
	subscriptions := make([]*deviceSubscription, 0, len(c.subscriptions))
	for id, subscription := range c.subscriptions {
		subscriptions = append(subscriptions, subscription)
		delete(c.subscriptions, id)
	}
	c.subscriptionsMu.Unlock()

	for _, subscription := range subscriptions {
		subscription.stop()
	}
}
