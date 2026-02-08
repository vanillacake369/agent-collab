package ast

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileWatcher는 파일 변경 감시자입니다.
type FileWatcher struct {
	mu           sync.RWMutex
	parser       *Parser
	differ       *Differ
	watchedFiles map[string]*WatchedFile
	callbacks    []ChangeCallback
	pollInterval time.Duration
	stopCh       chan struct{}
}

// WatchedFile은 감시 중인 파일입니다.
type WatchedFile struct {
	Path        string       `json:"path"`
	LastModTime time.Time    `json:"last_mod_time"`
	LastResult  *ParseResult `json:"last_result"`
}

// ChangeCallback은 변경 콜백입니다.
type ChangeCallback func(*FileChange) error

// FileChange는 파일 변경 정보입니다.
type FileChange struct {
	FilePath  string     `json:"file_path"`
	Type      ChangeType `json:"type"`
	Diff      *FileDiff  `json:"diff,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

// ChangeType은 변경 유형입니다.
type ChangeType string

const (
	ChangeCreated  ChangeType = "created"
	ChangeModified ChangeType = "modified"
	ChangeDeleted  ChangeType = "deleted"
)

// NewFileWatcher는 새 파일 감시자를 생성합니다.
func NewFileWatcher(pollInterval time.Duration) *FileWatcher {
	if pollInterval == 0 {
		pollInterval = 1 * time.Second
	}

	return &FileWatcher{
		parser:       NewParser(),
		differ:       NewDiffer(),
		watchedFiles: make(map[string]*WatchedFile),
		callbacks:    make([]ChangeCallback, 0),
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
	}
}

// Watch는 파일을 감시합니다.
func (w *FileWatcher) Watch(filePath string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	result, err := w.parser.ParseFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	w.mu.Lock()
	w.watchedFiles[absPath] = &WatchedFile{
		Path:        absPath,
		LastModTime: info.ModTime(),
		LastResult:  result,
	}
	w.mu.Unlock()

	return nil
}

// WatchDir는 디렉토리를 재귀적으로 감시합니다.
func (w *FileWatcher) WatchDir(dirPath string, extensions []string) error {
	extSet := make(map[string]bool)
	for _, ext := range extensions {
		extSet[ext] = true
	}

	return filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// .git 등 숨김 디렉토리 무시
			if len(info.Name()) > 0 && info.Name()[0] == '.' {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if len(extensions) > 0 && !extSet[ext] {
			return nil
		}

		// 지원하는 언어인지 확인
		if DetectLanguage(path) == LangUnknown {
			return nil
		}

		return w.Watch(path)
	})
}

// Unwatch는 파일 감시를 중단합니다.
func (w *FileWatcher) Unwatch(filePath string) {
	absPath, _ := filepath.Abs(filePath)
	w.mu.Lock()
	delete(w.watchedFiles, absPath)
	w.mu.Unlock()
}

// OnChange는 변경 콜백을 등록합니다.
func (w *FileWatcher) OnChange(callback ChangeCallback) {
	w.mu.Lock()
	w.callbacks = append(w.callbacks, callback)
	w.mu.Unlock()
}

// Start는 감시를 시작합니다.
func (w *FileWatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.checkChanges()
		}
	}
}

// Stop은 감시를 중단합니다.
func (w *FileWatcher) Stop() {
	close(w.stopCh)
}

// checkChanges는 변경을 확인합니다.
func (w *FileWatcher) checkChanges() {
	w.mu.RLock()
	files := make([]*WatchedFile, 0, len(w.watchedFiles))
	for _, f := range w.watchedFiles {
		files = append(files, f)
	}
	w.mu.RUnlock()

	for _, watched := range files {
		info, err := os.Stat(watched.Path)
		if err != nil {
			if os.IsNotExist(err) {
				// 삭제됨
				w.notifyChange(&FileChange{
					FilePath:  watched.Path,
					Type:      ChangeDeleted,
					Timestamp: time.Now(),
				})
				w.Unwatch(watched.Path)
			}
			continue
		}

		if info.ModTime().After(watched.LastModTime) {
			// 변경됨
			newResult, err := w.parser.ParseFile(watched.Path)
			if err != nil {
				continue
			}

			diff, err := w.differ.Diff(watched.LastResult, newResult)
			if err != nil {
				continue
			}

			if diff.HasChanges() {
				w.notifyChange(&FileChange{
					FilePath:  watched.Path,
					Type:      ChangeModified,
					Diff:      diff,
					Timestamp: time.Now(),
				})
			}

			w.mu.Lock()
			if wf, ok := w.watchedFiles[watched.Path]; ok {
				wf.LastModTime = info.ModTime()
				wf.LastResult = newResult
			}
			w.mu.Unlock()
		}
	}
}

// notifyChange는 변경을 알립니다.
func (w *FileWatcher) notifyChange(change *FileChange) {
	w.mu.RLock()
	callbacks := make([]ChangeCallback, len(w.callbacks))
	copy(callbacks, w.callbacks)
	w.mu.RUnlock()

	for _, cb := range callbacks {
		cb(change)
	}
}

// GetWatchedFiles는 감시 중인 파일 목록을 반환합니다.
func (w *FileWatcher) GetWatchedFiles() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()

	files := make([]string, 0, len(w.watchedFiles))
	for path := range w.watchedFiles {
		files = append(files, path)
	}
	return files
}

// GetParseResult는 파일의 파싱 결과를 반환합니다.
func (w *FileWatcher) GetParseResult(filePath string) (*ParseResult, error) {
	absPath, _ := filepath.Abs(filePath)

	w.mu.RLock()
	watched, ok := w.watchedFiles[absPath]
	w.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("file not watched: %s", filePath)
	}

	return watched.LastResult, nil
}
