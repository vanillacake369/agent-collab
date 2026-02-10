package ast

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Language는 프로그래밍 언어입니다.
type Language string

const (
	LangGo         Language = "go"
	LangTypeScript Language = "typescript"
	LangJavaScript Language = "javascript"
	LangPython     Language = "python"
	LangRust       Language = "rust"
	LangJava       Language = "java"
	LangUnknown    Language = "unknown"
)

// Parser는 AST 파서입니다.
type Parser struct {
	cache map[string]*ParseResult
}

// NewParser는 새 파서를 생성합니다.
func NewParser() *Parser {
	return &Parser{
		cache: make(map[string]*ParseResult),
	}
}

// ParseFile은 파일을 파싱합니다.
func (p *Parser) ParseFile(filePath string) (*ParseResult, error) {
	// #nosec G304 - filePath is provided by application code for code analysis, not direct user input
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lang := DetectLanguage(filePath)
	if lang == LangUnknown {
		return nil, fmt.Errorf("unsupported language for file: %s", filePath)
	}

	return p.Parse(filePath, string(content), lang)
}

// Parse는 소스 코드를 파싱합니다.
func (p *Parser) Parse(filePath, source string, lang Language) (*ParseResult, error) {
	// 캐시 확인
	hash := computeHash(source)
	cacheKey := fmt.Sprintf("%s:%s", filePath, hash)
	if cached, ok := p.cache[cacheKey]; ok {
		return cached, nil
	}

	// 간단한 패턴 기반 파싱 (tree-sitter 통합 전 임시 구현)
	var symbols []*Symbol
	var err error

	switch lang {
	case LangGo:
		symbols, err = parseGoSource(source)
	case LangTypeScript, LangJavaScript:
		symbols, err = parseJSSource(source)
	case LangPython:
		symbols, err = parsePythonSource(source)
	default:
		symbols, err = parseGenericSource(source)
	}

	if err != nil {
		return nil, err
	}

	result := &ParseResult{
		FilePath: filePath,
		Language: lang,
		Hash:     hash,
		Symbols:  symbols,
	}

	// 캐시 저장
	p.cache[cacheKey] = result

	return result, nil
}

// ClearCache는 캐시를 지웁니다.
func (p *Parser) ClearCache() {
	p.cache = make(map[string]*ParseResult)
}

// ParseResult는 파싱 결과입니다.
type ParseResult struct {
	FilePath string    `json:"file_path"`
	Language Language  `json:"language"`
	Hash     string    `json:"hash"`
	Symbols  []*Symbol `json:"symbols"`
}

// Symbol은 코드 심볼입니다.
type Symbol struct {
	Type      SymbolType `json:"type"`
	Name      string     `json:"name"`
	StartLine int        `json:"start_line"`
	EndLine   int        `json:"end_line"`
	StartCol  int        `json:"start_col"`
	EndCol    int        `json:"end_col"`
	Parent    string     `json:"parent,omitempty"`
	Children  []*Symbol  `json:"children,omitempty"`
	Hash      string     `json:"hash"`
}

// SymbolType은 심볼 유형입니다.
type SymbolType string

const (
	SymbolFunction  SymbolType = "function"
	SymbolMethod    SymbolType = "method"
	SymbolClass     SymbolType = "class"
	SymbolStruct    SymbolType = "struct"
	SymbolInterface SymbolType = "interface"
	SymbolVariable  SymbolType = "variable"
	SymbolConstant  SymbolType = "constant"
	SymbolTypeDef   SymbolType = "type"
	SymbolImport    SymbolType = "import"
)

// DetectLanguage는 파일 확장자로 언어를 감지합니다.
func DetectLanguage(filePath string) Language {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return LangGo
	case ".ts", ".tsx":
		return LangTypeScript
	case ".js", ".jsx", ".mjs":
		return LangJavaScript
	case ".py":
		return LangPython
	case ".rs":
		return LangRust
	case ".java":
		return LangJava
	default:
		return LangUnknown
	}
}

// computeHash는 소스의 해시를 계산합니다.
func computeHash(source string) string {
	hash := sha256.Sum256([]byte(source))
	return hex.EncodeToString(hash[:8])
}

// parseGoSource는 Go 소스를 파싱합니다.
func parseGoSource(source string) ([]*Symbol, error) {
	var symbols []*Symbol
	lines := strings.Split(source, "\n")

	var currentStruct string
	structStartLine := 0

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// 함수/메서드
		if strings.HasPrefix(trimmed, "func ") {
			name, isMethod, receiver := parseGoFunc(trimmed)
			if name != "" {
				symType := SymbolFunction
				parent := ""
				if isMethod {
					symType = SymbolMethod
					parent = receiver
				}
				endLine := findBlockEnd(lines, i)
				content := strings.Join(lines[i:endLine], "\n")
				symbols = append(symbols, &Symbol{
					Type:      symType,
					Name:      name,
					StartLine: lineNum,
					EndLine:   endLine,
					Parent:    parent,
					Hash:      computeHash(content),
				})
			}
		}

		// 구조체
		if strings.HasPrefix(trimmed, "type ") && strings.Contains(trimmed, "struct") {
			name := parseGoType(trimmed)
			if name != "" {
				currentStruct = name
				structStartLine = lineNum
			}
		}

		// 구조체 종료
		if currentStruct != "" && trimmed == "}" {
			content := strings.Join(lines[structStartLine-1:lineNum], "\n")
			symbols = append(symbols, &Symbol{
				Type:      SymbolStruct,
				Name:      currentStruct,
				StartLine: structStartLine,
				EndLine:   lineNum,
				Hash:      computeHash(content),
			})
			currentStruct = ""
		}

		// 인터페이스
		if strings.HasPrefix(trimmed, "type ") && strings.Contains(trimmed, "interface") {
			name := parseGoType(trimmed)
			if name != "" {
				endLine := findBlockEnd(lines, i)
				content := strings.Join(lines[i:endLine], "\n")
				symbols = append(symbols, &Symbol{
					Type:      SymbolInterface,
					Name:      name,
					StartLine: lineNum,
					EndLine:   endLine,
					Hash:      computeHash(content),
				})
			}
		}

		// 상수
		if strings.HasPrefix(trimmed, "const ") {
			name := parseConstVar(trimmed, "const ")
			if name != "" {
				symbols = append(symbols, &Symbol{
					Type:      SymbolConstant,
					Name:      name,
					StartLine: lineNum,
					EndLine:   lineNum,
					Hash:      computeHash(trimmed),
				})
			}
		}

		// 변수
		if strings.HasPrefix(trimmed, "var ") {
			name := parseConstVar(trimmed, "var ")
			if name != "" {
				symbols = append(symbols, &Symbol{
					Type:      SymbolVariable,
					Name:      name,
					StartLine: lineNum,
					EndLine:   lineNum,
					Hash:      computeHash(trimmed),
				})
			}
		}
	}

	return symbols, nil
}

// parseGoFunc는 Go 함수/메서드를 파싱합니다.
func parseGoFunc(line string) (name string, isMethod bool, receiver string) {
	line = strings.TrimPrefix(line, "func ")

	// 메서드: func (r *Receiver) Name(...)
	if strings.HasPrefix(line, "(") {
		parenEnd := strings.Index(line, ")")
		if parenEnd > 0 {
			receiverPart := line[1:parenEnd]
			parts := strings.Fields(receiverPart)
			if len(parts) >= 2 {
				receiver = strings.TrimPrefix(parts[1], "*")
			} else if len(parts) == 1 {
				receiver = strings.TrimPrefix(parts[0], "*")
			}
			line = strings.TrimSpace(line[parenEnd+1:])
			isMethod = true
		}
	}

	// 함수 이름 추출
	nameEnd := strings.Index(line, "(")
	if nameEnd > 0 {
		name = line[:nameEnd]
	}

	return name, isMethod, receiver
}

// parseGoType는 Go 타입 이름을 파싱합니다.
func parseGoType(line string) string {
	line = strings.TrimPrefix(line, "type ")
	parts := strings.Fields(line)
	if len(parts) >= 1 {
		return parts[0]
	}
	return ""
}

// parseConstVar는 const/var 이름을 파싱합니다.
func parseConstVar(line, prefix string) string {
	line = strings.TrimPrefix(line, prefix)
	// const ( 형태 무시
	if strings.HasPrefix(line, "(") {
		return ""
	}
	parts := strings.Fields(line)
	if len(parts) >= 1 {
		return strings.TrimSuffix(parts[0], "=")
	}
	return ""
}

// parseJSSource는 JavaScript/TypeScript 소스를 파싱합니다.
func parseJSSource(source string) ([]*Symbol, error) {
	var symbols []*Symbol
	lines := strings.Split(source, "\n")

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// 함수
		if strings.HasPrefix(trimmed, "function ") || strings.Contains(trimmed, "function(") {
			name := parseJSFunc(trimmed)
			if name != "" {
				endLine := findBlockEnd(lines, i)
				content := strings.Join(lines[i:endLine], "\n")
				symbols = append(symbols, &Symbol{
					Type:      SymbolFunction,
					Name:      name,
					StartLine: lineNum,
					EndLine:   endLine,
					Hash:      computeHash(content),
				})
			}
		}

		// 화살표 함수
		if strings.Contains(trimmed, "=>") && (strings.HasPrefix(trimmed, "const ") || strings.HasPrefix(trimmed, "let ")) {
			name := parseArrowFunc(trimmed)
			if name != "" {
				endLine := findBlockEnd(lines, i)
				content := strings.Join(lines[i:endLine], "\n")
				symbols = append(symbols, &Symbol{
					Type:      SymbolFunction,
					Name:      name,
					StartLine: lineNum,
					EndLine:   endLine,
					Hash:      computeHash(content),
				})
			}
		}

		// 클래스
		if strings.HasPrefix(trimmed, "class ") {
			name := parseJSClass(trimmed)
			if name != "" {
				endLine := findBlockEnd(lines, i)
				content := strings.Join(lines[i:endLine], "\n")
				symbols = append(symbols, &Symbol{
					Type:      SymbolClass,
					Name:      name,
					StartLine: lineNum,
					EndLine:   endLine,
					Hash:      computeHash(content),
				})
			}
		}

		// 인터페이스 (TypeScript)
		if strings.HasPrefix(trimmed, "interface ") {
			name := parseJSInterface(trimmed)
			if name != "" {
				endLine := findBlockEnd(lines, i)
				content := strings.Join(lines[i:endLine], "\n")
				symbols = append(symbols, &Symbol{
					Type:      SymbolInterface,
					Name:      name,
					StartLine: lineNum,
					EndLine:   endLine,
					Hash:      computeHash(content),
				})
			}
		}
	}

	return symbols, nil
}

// parseJSFunc는 JavaScript 함수를 파싱합니다.
func parseJSFunc(line string) string {
	line = strings.TrimPrefix(line, "function ")
	nameEnd := strings.Index(line, "(")
	if nameEnd > 0 {
		return strings.TrimSpace(line[:nameEnd])
	}
	return ""
}

// parseArrowFunc는 화살표 함수를 파싱합니다.
func parseArrowFunc(line string) string {
	line = strings.TrimPrefix(line, "const ")
	line = strings.TrimPrefix(line, "let ")
	eqIdx := strings.Index(line, "=")
	if eqIdx > 0 {
		return strings.TrimSpace(line[:eqIdx])
	}
	return ""
}

// parseJSClass는 JavaScript 클래스를 파싱합니다.
func parseJSClass(line string) string {
	line = strings.TrimPrefix(line, "class ")
	// extends 또는 { 전까지
	for _, sep := range []string{" extends ", " implements ", " {"} {
		if idx := strings.Index(line, sep); idx > 0 {
			return strings.TrimSpace(line[:idx])
		}
	}
	return strings.TrimSpace(strings.TrimSuffix(line, "{"))
}

// parseJSInterface는 TypeScript 인터페이스를 파싱합니다.
func parseJSInterface(line string) string {
	line = strings.TrimPrefix(line, "interface ")
	for _, sep := range []string{" extends ", " {"} {
		if idx := strings.Index(line, sep); idx > 0 {
			return strings.TrimSpace(line[:idx])
		}
	}
	return strings.TrimSpace(strings.TrimSuffix(line, "{"))
}

// parsePythonSource는 Python 소스를 파싱합니다.
func parsePythonSource(source string) ([]*Symbol, error) {
	var symbols []*Symbol
	lines := strings.Split(source, "\n")

	var currentClass string
	classStartLine := 0
	classIndent := 0

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))

		// 클래스 종료 확인
		if currentClass != "" && trimmed != "" && indent <= classIndent {
			content := strings.Join(lines[classStartLine-1:i], "\n")
			symbols = append(symbols, &Symbol{
				Type:      SymbolClass,
				Name:      currentClass,
				StartLine: classStartLine,
				EndLine:   i,
				Hash:      computeHash(content),
			})
			currentClass = ""
		}

		// 클래스
		if strings.HasPrefix(trimmed, "class ") {
			name := parsePythonClass(trimmed)
			if name != "" {
				currentClass = name
				classStartLine = lineNum
				classIndent = indent
			}
		}

		// 함수/메서드
		if strings.HasPrefix(trimmed, "def ") {
			name := parsePythonFunc(trimmed)
			if name != "" {
				symType := SymbolFunction
				parent := ""
				if currentClass != "" && indent > classIndent {
					symType = SymbolMethod
					parent = currentClass
				}
				endLine := findPythonBlockEnd(lines, i, indent)
				content := strings.Join(lines[i:endLine], "\n")
				symbols = append(symbols, &Symbol{
					Type:      symType,
					Name:      name,
					StartLine: lineNum,
					EndLine:   endLine,
					Parent:    parent,
					Hash:      computeHash(content),
				})
			}
		}
	}

	// 마지막 클래스 처리
	if currentClass != "" {
		content := strings.Join(lines[classStartLine-1:], "\n")
		symbols = append(symbols, &Symbol{
			Type:      SymbolClass,
			Name:      currentClass,
			StartLine: classStartLine,
			EndLine:   len(lines),
			Hash:      computeHash(content),
		})
	}

	return symbols, nil
}

// parsePythonClass는 Python 클래스를 파싱합니다.
func parsePythonClass(line string) string {
	line = strings.TrimPrefix(line, "class ")
	for _, sep := range []string{"(", ":"} {
		if idx := strings.Index(line, sep); idx > 0 {
			return strings.TrimSpace(line[:idx])
		}
	}
	return strings.TrimSpace(line)
}

// parsePythonFunc는 Python 함수를 파싱합니다.
func parsePythonFunc(line string) string {
	line = strings.TrimPrefix(line, "def ")
	if idx := strings.Index(line, "("); idx > 0 {
		return strings.TrimSpace(line[:idx])
	}
	return ""
}

// parseGenericSource는 범용 소스를 파싱합니다.
func parseGenericSource(source string) ([]*Symbol, error) {
	// 최소한의 파싱: 파일 전체를 하나의 심볼로
	lines := strings.Split(source, "\n")
	return []*Symbol{
		{
			Type:      SymbolTypeDef,
			Name:      "file",
			StartLine: 1,
			EndLine:   len(lines),
			Hash:      computeHash(source),
		},
	}, nil
}

// findBlockEnd는 블록의 끝을 찾습니다 (중괄호 기반).
func findBlockEnd(lines []string, startIdx int) int {
	depth := 0
	started := false

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		for _, ch := range line {
			if ch == '{' {
				depth++
				started = true
			} else if ch == '}' {
				depth--
				if started && depth == 0 {
					return i + 1
				}
			}
		}
	}

	return len(lines)
}

// findPythonBlockEnd는 Python 블록의 끝을 찾습니다 (들여쓰기 기반).
func findPythonBlockEnd(lines []string, startIdx, baseIndent int) int {
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent <= baseIndent {
			return i
		}
	}
	return len(lines)
}
