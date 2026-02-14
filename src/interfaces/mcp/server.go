package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"agent-collab/src/domain/agent"
)

// Server is an MCP server that allows external agents to connect.
type Server struct {
	mu sync.RWMutex

	// Server info
	name    string
	version string

	// Tool handlers
	tools    map[string]ToolHandler
	toolList []Tool

	// Resource handlers
	resources    map[string]ResourceHandler
	resourceList []Resource

	// Agent registry
	registry *agent.Registry

	// IO
	reader *bufio.Reader
	writer io.Writer

	// State
	initialized bool
	clientInfo  Implementation

	ctx    context.Context
	cancel context.CancelFunc
}

// ToolHandler handles a tool call.
type ToolHandler func(ctx context.Context, args map[string]any) (*ToolCallResult, error)

// ResourceHandler handles a resource read.
type ResourceHandler func(ctx context.Context, uri string) (*ReadResourceResult, error)

// NewServer creates a new MCP server.
func NewServer(name, version string, registry *agent.Registry) *Server {
	return &Server{
		name:      name,
		version:   version,
		tools:     make(map[string]ToolHandler),
		toolList:  make([]Tool, 0),
		resources: make(map[string]ResourceHandler),
		registry:  registry,
	}
}

// RegisterTool registers a tool.
func (s *Server) RegisterTool(tool Tool, handler ToolHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tools[tool.Name] = handler
	s.toolList = append(s.toolList, tool)
}

// RegisterResource registers a resource.
func (s *Server) RegisterResource(resource Resource, handler ResourceHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.resources[resource.URI] = handler
	s.resourceList = append(s.resourceList, resource)
}

// ServeStdio starts the server on stdin/stdout.
func (s *Server) ServeStdio(ctx context.Context) error {
	return s.Serve(ctx, os.Stdin, os.Stdout)
}

// Serve starts the server with custom reader/writer.
func (s *Server) Serve(ctx context.Context, reader io.Reader, writer io.Writer) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.reader = bufio.NewReader(reader)
	s.writer = writer

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
			if err := s.handleMessage(); err != nil {
				if err == io.EOF {
					return nil
				}
				// Log error but continue
				fmt.Fprintf(os.Stderr, "MCP error: %v\n", err)
			}
		}
	}
}

// Close stops the server.
func (s *Server) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

func (s *Server) handleMessage() error {
	line, err := s.reader.ReadBytes('\n')
	if err != nil {
		return err
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(line, &req); err != nil {
		return s.sendError(nil, ErrorCodeParseError, "Parse error", nil)
	}

	return s.dispatch(&req)
}

func (s *Server) dispatch(req *JSONRPCRequest) error {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Client notification that initialization is complete
		return nil
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(req)
	case "resources/list":
		return s.handleListResources(req)
	case "resources/read":
		return s.handleReadResource(req)
	case "ping":
		return s.sendResult(req.ID, map[string]string{})
	default:
		return s.sendError(req.ID, ErrorCodeMethodNotFound, "Method not found", nil)
	}
}

func (s *Server) handleInitialize(req *JSONRPCRequest) error {
	var params InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.sendError(req.ID, ErrorCodeInvalidParams, "Invalid params", nil)
	}

	s.mu.Lock()
	s.initialized = true
	s.clientInfo = params.ClientInfo
	s.mu.Unlock()

	// Register the client as an agent if we have a registry
	if s.registry != nil {
		agentInfo := agent.AgentInfo{
			ID:       fmt.Sprintf("mcp-%s-%s", params.ClientInfo.Name, params.ClientInfo.Version),
			Name:     params.ClientInfo.Name,
			Provider: agent.ProviderCustom,
			Model:    "unknown",
			Version:  params.ClientInfo.Version,
			Capabilities: []agent.Capability{
				agent.CapabilityToolUse,
			},
		}
		s.registry.Register(&agent.ConnectedAgent{
			Info:   agentInfo,
			PeerID: "mcp-stdio",
			Status: agent.StatusOnline,
		})
	}

	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: Capabilities{
			Tools: &ToolCapabilities{
				ListChanged: true,
			},
			Resources: &ResourceCapabilities{
				Subscribe:   false,
				ListChanged: true,
			},
		},
		ServerInfo: Implementation{
			Name:    s.name,
			Version: s.version,
		},
	}

	return s.sendResult(req.ID, result)
}

func (s *Server) handleListTools(req *JSONRPCRequest) error {
	s.mu.RLock()
	tools := s.toolList
	s.mu.RUnlock()

	return s.sendResult(req.ID, ListToolsResult{Tools: tools})
}

func (s *Server) handleCallTool(req *JSONRPCRequest) error {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.sendError(req.ID, ErrorCodeInvalidParams, "Invalid params", nil)
	}

	s.mu.RLock()
	handler, ok := s.tools[params.Name]
	s.mu.RUnlock()

	if !ok {
		return s.sendError(req.ID, ErrorCodeMethodNotFound, "Tool not found", nil)
	}

	result, err := handler(s.ctx, params.Arguments)
	if err != nil {
		return s.sendResult(req.ID, ToolCallResult{
			Content: []Content{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
	}

	return s.sendResult(req.ID, result)
}

func (s *Server) handleListResources(req *JSONRPCRequest) error {
	s.mu.RLock()
	resources := s.resourceList
	s.mu.RUnlock()

	return s.sendResult(req.ID, ListResourcesResult{Resources: resources})
}

func (s *Server) handleReadResource(req *JSONRPCRequest) error {
	var params ReadResourceParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.sendError(req.ID, ErrorCodeInvalidParams, "Invalid params", nil)
	}

	s.mu.RLock()
	handler, ok := s.resources[params.URI]
	s.mu.RUnlock()

	if !ok {
		return s.sendError(req.ID, ErrorCodeInvalidParams, "Resource not found", nil)
	}

	result, err := handler(s.ctx, params.URI)
	if err != nil {
		return s.sendError(req.ID, ErrorCodeInternalError, err.Error(), nil)
	}

	return s.sendResult(req.ID, result)
}

func (s *Server) sendResult(id any, result any) error {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	return s.send(resp)
}

func (s *Server) sendError(id any, code int, message string, data any) error {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	return s.send(resp)
}

func (s *Server) send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(s.writer, "%s\n", data)
	return err
}
