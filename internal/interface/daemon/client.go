package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Client is a client for communicating with the daemon.
type Client struct {
	socketPath  string
	httpClient  *http.Client
	eventClient *EventClient
}

// NewClient creates a new daemon client.
func NewClient() *Client {
	socketPath := DefaultSocketPath()

	// Create HTTP client with Unix socket transport
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}

	return &Client{
		socketPath:  socketPath,
		eventClient: NewEventClient(),
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

// SubscribeEvents connects to the event stream and returns event/error channels.
func (c *Client) SubscribeEvents(ctx context.Context) (<-chan Event, <-chan error, error) {
	if err := c.eventClient.Connect(ctx); err != nil {
		return nil, nil, err
	}
	return c.eventClient.Events(), c.eventClient.Errors(), nil
}

// SubscribeEventsWithRetry connects to the event stream with automatic reconnection.
func (c *Client) SubscribeEventsWithRetry(ctx context.Context) <-chan Event {
	c.eventClient.ConnectWithRetry(ctx)
	return c.eventClient.Events()
}

// CloseEvents closes the event subscription.
func (c *Client) CloseEvents() error {
	return c.eventClient.Close()
}

// IsRunning checks if the daemon is running.
func (c *Client) IsRunning() bool {
	// Check if socket exists
	if _, err := os.Stat(c.socketPath); os.IsNotExist(err) {
		return false
	}

	// Try to get status
	_, err := c.Status()
	return err == nil
}

// GetPID returns the daemon PID if running.
func (c *Client) GetPID() (int, error) {
	pidFile := DefaultPIDFile()
	// #nosec G304 - pidFile is from DefaultPIDFile() which returns a fixed path in user's home directory
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

// Status returns the daemon status.
func (c *Client) Status() (*StatusResponse, error) {
	resp, err := c.get("/status")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}

// Init initializes a new cluster.
func (c *Client) Init(projectName string) (*InitResponse, error) {
	resp, err := c.post("/init", InitRequest{ProjectName: projectName})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result InitResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("%s", result.Error)
	}
	return &result, nil
}

// Join joins an existing cluster.
func (c *Client) Join(token string) (*JoinResponse, error) {
	resp, err := c.post("/join", JoinRequest{Token: token})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result JoinResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("%s", result.Error)
	}
	return &result, nil
}

// AcquireLock acquires a lock.
func (c *Client) AcquireLock(filePath string, startLine, endLine int, intention string) (*LockResponse, error) {
	resp, err := c.post("/lock/acquire", LockRequest{
		FilePath:  filePath,
		StartLine: startLine,
		EndLine:   endLine,
		Intention: intention,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result LockResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReleaseLock releases a lock.
func (c *Client) ReleaseLock(lockID string) error {
	resp, err := c.post("/lock/release", ReleaseLockRequest{LockID: lockID})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result GenericResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}

// ListLocks returns all active locks.
func (c *Client) ListLocks() (*ListLocksResponse, error) {
	resp, err := c.get("/lock/list")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ListLocksResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Embed generates embeddings for text.
func (c *Client) Embed(text string) (*EmbedResponse, error) {
	resp, err := c.post("/embed", EmbedRequest{Text: text})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result EmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Search searches for similar content.
func (c *Client) Search(query string, limit int) (*SearchResponse, error) {
	resp, err := c.post("/search", SearchRequest{Query: query, Limit: limit})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListAgents returns connected agents.
func (c *Client) ListAgents() (*ListAgentsResponse, error) {
	resp, err := c.get("/agents/list")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ListAgentsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListPeers returns connected peers.
func (c *Client) ListPeers() (*ListPeersResponse, error) {
	resp, err := c.get("/peers/list")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ListPeersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// WatchFile starts watching a file.
func (c *Client) WatchFile(filePath string) error {
	resp, err := c.post("/context/watch", WatchFileRequest{FilePath: filePath})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result GenericResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}
	if result.Error != "" {
		return fmt.Errorf("%s", result.Error)
	}
	return nil
}

// ListEventsResponse is the response for listing events.
type ListEventsResponse struct {
	Events []Event `json:"events"`
	Count  int     `json:"count"`
}

// ListEvents returns recent events from the daemon.
func (c *Client) ListEvents(limit int, eventType string) (*ListEventsResponse, error) {
	path := fmt.Sprintf("/events/list?limit=%d", limit)
	if eventType != "" {
		path += "&type=" + eventType
	}

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ListEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ShareContext shares context content with the cluster and stores in vector DB.
func (c *Client) ShareContext(filePath, content string, metadata map[string]any) (*ShareContextResponse, error) {
	resp, err := c.post("/context/share", ShareContextRequest{
		FilePath: filePath,
		Content:  content,
		Metadata: metadata,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ShareContextResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("%s", result.Error)
	}
	return &result, nil
}

// Shutdown shuts down the daemon.
func (c *Client) Shutdown() error {
	resp, err := c.post("/shutdown", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result GenericResponse
	json.NewDecoder(resp.Body).Decode(&result)
	return nil
}

func (c *Client) get(path string) (*http.Response, error) {
	return c.httpClient.Get("http://unix" + path)
}

func (c *Client) post(path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	return c.httpClient.Post("http://unix"+path, "application/json", reader)
}
