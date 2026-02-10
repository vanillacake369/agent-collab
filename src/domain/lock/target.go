package lock

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// TargetType is the semantic target type.
type TargetType string

const (
	TargetFunction TargetType = "function"
	TargetClass    TargetType = "class"
	TargetMethod   TargetType = "method"
	TargetModule   TargetType = "module"
	TargetFile     TargetType = "file"
)

// SemanticTarget is the lock target.
type SemanticTarget struct {
	Type      TargetType `json:"type"`
	FilePath  string     `json:"file_path"`
	Name      string     `json:"name"`
	StartLine int        `json:"start_line"`
	EndLine   int        `json:"end_line"`
	ASTHash   string     `json:"ast_hash"`
}

// NewSemanticTarget creates a new semantic target with validation.
func NewSemanticTarget(targetType TargetType, filePath, name string, startLine, endLine int) (*SemanticTarget, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}
	if startLine <= 0 {
		return nil, fmt.Errorf("start line must be positive: %d", startLine)
	}
	if endLine <= 0 {
		return nil, fmt.Errorf("end line must be positive: %d", endLine)
	}
	if startLine > endLine {
		return nil, fmt.Errorf("start line (%d) must be <= end line (%d)", startLine, endLine)
	}
	if !isValidTargetType(targetType) {
		return nil, fmt.Errorf("invalid target type: %s", targetType)
	}

	return &SemanticTarget{
		Type:      targetType,
		FilePath:  filePath,
		Name:      name,
		StartLine: startLine,
		EndLine:   endLine,
	}, nil
}

// isValidTargetType checks if the target type is valid.
func isValidTargetType(t TargetType) bool {
	switch t {
	case TargetFunction, TargetClass, TargetMethod, TargetModule, TargetFile:
		return true
	default:
		return false
	}
}

// ID returns the unique ID of the target.
func (t *SemanticTarget) ID() string {
	return fmt.Sprintf("%s:%s:%s:%d-%d",
		t.Type, t.FilePath, t.Name, t.StartLine, t.EndLine)
}

// SetASTHash sets the AST hash.
func (t *SemanticTarget) SetASTHash(content []byte) {
	hash := sha256.Sum256(content)
	t.ASTHash = hex.EncodeToString(hash[:8])
}

// Overlaps checks if two targets overlap.
func (t *SemanticTarget) Overlaps(other *SemanticTarget) bool {
	if t.FilePath != other.FilePath {
		return false
	}

	// Check if line ranges overlap
	return t.StartLine <= other.EndLine && other.StartLine <= t.EndLine
}

// Contains checks if this target contains another target.
func (t *SemanticTarget) Contains(other *SemanticTarget) bool {
	if t.FilePath != other.FilePath {
		return false
	}

	return t.StartLine <= other.StartLine && t.EndLine >= other.EndLine
}

// String returns the string representation of the target.
func (t *SemanticTarget) String() string {
	return fmt.Sprintf("%s %s (%s:%d-%d)",
		t.Type, t.Name, t.FilePath, t.StartLine, t.EndLine)
}
