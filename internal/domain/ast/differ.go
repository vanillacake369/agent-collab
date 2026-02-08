package ast

import (
	"fmt"
)

// DiffType은 차이 유형입니다.
type DiffType string

const (
	DiffAdded    DiffType = "added"
	DiffRemoved  DiffType = "removed"
	DiffModified DiffType = "modified"
	DiffMoved    DiffType = "moved"
)

// SymbolDiff는 심볼 차이입니다.
type SymbolDiff struct {
	Type       DiffType `json:"type"`
	Symbol     *Symbol  `json:"symbol"`
	OldSymbol  *Symbol  `json:"old_symbol,omitempty"`
	OldLine    int      `json:"old_line,omitempty"`
	NewLine    int      `json:"new_line,omitempty"`
	HashBefore string   `json:"hash_before,omitempty"`
	HashAfter  string   `json:"hash_after,omitempty"`
}

// FileDiff는 파일 차이입니다.
type FileDiff struct {
	FilePath   string        `json:"file_path"`
	Language   Language      `json:"language"`
	OldHash    string        `json:"old_hash"`
	NewHash    string        `json:"new_hash"`
	Diffs      []*SymbolDiff `json:"diffs"`
	AddedCount int           `json:"added_count"`
	RemovedCount int         `json:"removed_count"`
	ModifiedCount int        `json:"modified_count"`
}

// Differ는 AST 차이 비교기입니다.
type Differ struct{}

// NewDiffer는 새 차이 비교기를 생성합니다.
func NewDiffer() *Differ {
	return &Differ{}
}

// Diff는 두 파싱 결과의 차이를 계산합니다.
func (d *Differ) Diff(old, new *ParseResult) (*FileDiff, error) {
	if old.FilePath != new.FilePath {
		return nil, fmt.Errorf("file paths don't match: %s vs %s", old.FilePath, new.FilePath)
	}

	diff := &FileDiff{
		FilePath: new.FilePath,
		Language: new.Language,
		OldHash:  old.Hash,
		NewHash:  new.Hash,
		Diffs:    make([]*SymbolDiff, 0),
	}

	// 해시가 같으면 변경 없음
	if old.Hash == new.Hash {
		return diff, nil
	}

	// 심볼 맵 생성
	oldSymbols := make(map[string]*Symbol)
	for _, sym := range old.Symbols {
		key := d.symbolKey(sym)
		oldSymbols[key] = sym
	}

	newSymbols := make(map[string]*Symbol)
	for _, sym := range new.Symbols {
		key := d.symbolKey(sym)
		newSymbols[key] = sym
	}

	// 추가/수정된 심볼 찾기
	for key, newSym := range newSymbols {
		if oldSym, exists := oldSymbols[key]; exists {
			// 수정 확인
			if oldSym.Hash != newSym.Hash {
				diff.Diffs = append(diff.Diffs, &SymbolDiff{
					Type:       DiffModified,
					Symbol:     newSym,
					OldSymbol:  oldSym,
					OldLine:    oldSym.StartLine,
					NewLine:    newSym.StartLine,
					HashBefore: oldSym.Hash,
					HashAfter:  newSym.Hash,
				})
				diff.ModifiedCount++
			} else if oldSym.StartLine != newSym.StartLine {
				// 이동됨
				diff.Diffs = append(diff.Diffs, &SymbolDiff{
					Type:    DiffMoved,
					Symbol:  newSym,
					OldLine: oldSym.StartLine,
					NewLine: newSym.StartLine,
				})
			}
		} else {
			// 추가됨
			diff.Diffs = append(diff.Diffs, &SymbolDiff{
				Type:      DiffAdded,
				Symbol:    newSym,
				NewLine:   newSym.StartLine,
				HashAfter: newSym.Hash,
			})
			diff.AddedCount++
		}
	}

	// 삭제된 심볼 찾기
	for key, oldSym := range oldSymbols {
		if _, exists := newSymbols[key]; !exists {
			diff.Diffs = append(diff.Diffs, &SymbolDiff{
				Type:       DiffRemoved,
				Symbol:     oldSym,
				OldLine:    oldSym.StartLine,
				HashBefore: oldSym.Hash,
			})
			diff.RemovedCount++
		}
	}

	return diff, nil
}

// symbolKey는 심볼의 고유 키를 생성합니다.
func (d *Differ) symbolKey(sym *Symbol) string {
	return fmt.Sprintf("%s:%s:%s", sym.Type, sym.Parent, sym.Name)
}

// HasChanges는 변경이 있는지 확인합니다.
func (fd *FileDiff) HasChanges() bool {
	return len(fd.Diffs) > 0
}

// GetModifiedSymbols는 수정된 심볼을 반환합니다.
func (fd *FileDiff) GetModifiedSymbols() []*SymbolDiff {
	var result []*SymbolDiff
	for _, d := range fd.Diffs {
		if d.Type == DiffModified {
			result = append(result, d)
		}
	}
	return result
}

// GetAddedSymbols는 추가된 심볼을 반환합니다.
func (fd *FileDiff) GetAddedSymbols() []*SymbolDiff {
	var result []*SymbolDiff
	for _, d := range fd.Diffs {
		if d.Type == DiffAdded {
			result = append(result, d)
		}
	}
	return result
}

// GetRemovedSymbols는 삭제된 심볼을 반환합니다.
func (fd *FileDiff) GetRemovedSymbols() []*SymbolDiff {
	var result []*SymbolDiff
	for _, d := range fd.Diffs {
		if d.Type == DiffRemoved {
			result = append(result, d)
		}
	}
	return result
}
