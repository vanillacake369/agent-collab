package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// EventClient connects to the daemon event stream.
type EventClient struct {
	mu sync.Mutex

	socketPath string
	conn       net.Conn
	decoder    *json.Decoder
	reader     *bufio.Reader

	connected atomic.Bool
	eventCh   chan Event
	errorCh   chan error

	ctx    context.Context
	cancel context.CancelFunc

	// Reconnection settings
	reconnectInterval time.Duration
	maxReconnectDelay time.Duration
}

// NewEventClient creates a new event client.
func NewEventClient() *EventClient {
	return &EventClient{
		socketPath:        DefaultEventSocketPath(),
		eventCh:           make(chan Event, 64),
		errorCh:           make(chan error, 1),
		reconnectInterval: 1 * time.Second,
		maxReconnectDelay: 30 * time.Second,
	}
}

// Connect connects to the event stream.
func (c *EventClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ctx, c.cancel = context.WithCancel(ctx)

	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("failed to connect to event socket: %w", err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.decoder = json.NewDecoder(c.reader)
	c.connected.Store(true)

	// Start reading events
	go c.readLoop()

	return nil
}

// ConnectWithRetry connects with automatic reconnection.
func (c *EventClient) ConnectWithRetry(ctx context.Context) {
	c.ctx, c.cancel = context.WithCancel(ctx)

	go func() {
		delay := c.reconnectInterval
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
			}

			err := c.connect()
			if err == nil {
				delay = c.reconnectInterval // Reset delay on success
				c.readLoop()                // Block until disconnection
			}

			if !c.connected.Load() {
				// Disconnected, try to reconnect
				select {
				case <-c.ctx.Done():
					return
				case <-time.After(delay):
					// Exponential backoff
					delay = delay * 2
					if delay > c.maxReconnectDelay {
						delay = c.maxReconnectDelay
					}
				}
			}
		}
	}()
}

func (c *EventClient) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return err
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.decoder = json.NewDecoder(c.reader)
	c.connected.Store(true)

	return nil
}

// Close closes the connection.
func (c *EventClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected.Store(false)

	if c.cancel != nil {
		c.cancel()
	}

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// Events returns a channel that receives events.
func (c *EventClient) Events() <-chan Event {
	return c.eventCh
}

// Errors returns a channel that receives errors.
func (c *EventClient) Errors() <-chan error {
	return c.errorCh
}

// IsConnected returns true if connected to the event stream.
func (c *EventClient) IsConnected() bool {
	return c.connected.Load()
}

func (c *EventClient) readLoop() {
	for c.connected.Load() {
		var event Event
		if err := c.decoder.Decode(&event); err != nil {
			c.connected.Store(false)
			c.mu.Lock()
			if c.conn != nil {
				c.conn.Close()
				c.conn = nil
			}
			c.mu.Unlock()

			select {
			case c.errorCh <- err:
			default:
			}
			return
		}

		select {
		case c.eventCh <- event:
		default:
			// Channel full, drop event
		}
	}
}

// OnEvent registers a callback for a specific event type.
// Returns a function to stop listening.
func (c *EventClient) OnEvent(eventType EventType, callback func(Event)) func() {
	ctx, cancel := context.WithCancel(c.ctx)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-c.eventCh:
				if event.Type == eventType {
					callback(event)
				}
			}
		}
	}()

	return cancel
}

// WaitForEvent blocks until an event of the given type is received or context is canceled.
func (c *EventClient) WaitForEvent(ctx context.Context, eventType EventType) (*Event, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case event := <-c.eventCh:
			if event.Type == eventType {
				return &event, nil
			}
		}
	}
}
