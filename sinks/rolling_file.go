package sinks

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	
	"github.com/willibrandon/mtlog/core"
	"github.com/willibrandon/mtlog/internal/formatters"
)

// RollingInterval defines when to roll files based on time.
type RollingInterval int

const (
	// RollingIntervalNone disables time-based rolling.
	RollingIntervalNone RollingInterval = iota
	// RollingIntervalHourly rolls files every hour.
	RollingIntervalHourly
	// RollingIntervalDaily rolls files every day.
	RollingIntervalDaily
	// RollingIntervalWeekly rolls files every week.
	RollingIntervalWeekly
	// RollingIntervalMonthly rolls files every month.
	RollingIntervalMonthly
)

// RollingFileOptions configures the rolling file sink.
type RollingFileOptions struct {
	// FilePath is the path to the log file.
	FilePath string
	
	// MaxFileSize is the maximum size of a file before rolling (in bytes).
	// 0 means no size limit.
	MaxFileSize int64
	
	// RollingInterval defines time-based rolling.
	RollingInterval RollingInterval
	
	// RetainFileCount is the number of rolled files to keep.
	// 0 means keep all files.
	RetainFileCount int
	
	// CompressRolledFiles enables gzip compression for rolled files.
	CompressRolledFiles bool
	
	// Formatter to use for formatting log events.
	Formatter interface {
		Format(event *core.LogEvent) ([]byte, error)
	}
	
	// BufferSize for file writes (default 64KB).
	BufferSize int
}

// RollingFileSink writes log events to files with rolling support.
type RollingFileSink struct {
	options        RollingFileOptions
	file           *os.File
	writer         *bufferedWriter
	mu             sync.Mutex
	currentSize    int64
	rollTime       time.Time
	fileNameFormat string
	baseFileName   string
	fileExt        string
}

// NewRollingFileSink creates a new rolling file sink.
func NewRollingFileSink(options RollingFileOptions) (*RollingFileSink, error) {
	if options.FilePath == "" {
		return nil, fmt.Errorf("file path is required")
	}
	
	if options.BufferSize <= 0 {
		options.BufferSize = 64 * 1024 // 64KB default
	}
	
	if options.Formatter == nil {
		options.Formatter = formatters.NewCLEFFormatter()
	}
	
	// Parse the file path
	dir := filepath.Dir(options.FilePath)
	fileName := filepath.Base(options.FilePath)
	ext := filepath.Ext(fileName)
	baseName := strings.TrimSuffix(fileName, ext)
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}
	
	sink := &RollingFileSink{
		options:        options,
		baseFileName:   baseName,
		fileExt:        ext,
		fileNameFormat: filepath.Join(dir, baseName+"-%s"+ext),
	}
	
	// Open the initial file
	if err := sink.openFile(); err != nil {
		return nil, err
	}
	
	// Set initial roll time
	sink.updateRollTime()
	
	return sink, nil
}

// Emit writes a log event to the file.
func (rfs *RollingFileSink) Emit(event *core.LogEvent) {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()
	
	// Check if we need to roll
	if rfs.shouldRoll() {
		if err := rfs.roll(); err != nil {
			// Log rolling failed, but continue logging
			fmt.Fprintf(os.Stderr, "Failed to roll log file: %v\n", err)
		}
	}
	
	// Format the event
	data, err := rfs.options.Formatter.Format(event)
	if err != nil {
		return
	}
	
	// Write to file
	n, err := rfs.writer.Write(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
		return
	}
	
	// Write newline
	if _, err := rfs.writer.Write([]byte{'\n'}); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write newline to log file: %v\n", err)
		return
	}
	
	rfs.currentSize += int64(n) + 1
}

// Close closes the file sink.
func (rfs *RollingFileSink) Close() error {
	rfs.mu.Lock()
	defer rfs.mu.Unlock()
	
	if rfs.writer != nil {
		if err := rfs.writer.Flush(); err != nil {
			return err
		}
	}
	
	if rfs.file != nil {
		return rfs.file.Close()
	}
	
	return nil
}

// openFile opens the log file for writing.
func (rfs *RollingFileSink) openFile() error {
	file, err := os.OpenFile(rfs.options.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	
	// Get current file size
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat file: %w", err)
	}
	
	rfs.file = file
	rfs.writer = newBufferedWriter(file, rfs.options.BufferSize)
	rfs.currentSize = stat.Size()
	
	return nil
}

// shouldRoll checks if the file should be rolled.
func (rfs *RollingFileSink) shouldRoll() bool {
	// Check size limit
	if rfs.options.MaxFileSize > 0 && rfs.currentSize >= rfs.options.MaxFileSize {
		return true
	}
	
	// Check time-based rolling
	if rfs.options.RollingInterval != RollingIntervalNone {
		return time.Now().After(rfs.rollTime)
	}
	
	return false
}

// roll performs the file rolling.
func (rfs *RollingFileSink) roll() error {
	// Flush and close current file
	if rfs.writer != nil {
		if err := rfs.writer.Flush(); err != nil {
			return err
		}
	}
	
	if rfs.file != nil {
		if err := rfs.file.Close(); err != nil {
			return err
		}
	}
	
	// Generate rolled file name
	timestamp := time.Now().Format("20060102-150405")
	rolledPath := fmt.Sprintf(rfs.fileNameFormat, timestamp)
	
	// Rename current file
	if err := os.Rename(rfs.options.FilePath, rolledPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}
	
	// Compress if enabled
	if rfs.options.CompressRolledFiles {
		if err := rfs.compressFile(rolledPath); err != nil {
			// Compression failed, but file is already rolled
			fmt.Fprintf(os.Stderr, "Failed to compress rolled file: %v\n", err)
		}
	}
	
	// Clean up old files
	if rfs.options.RetainFileCount > 0 {
		if err := rfs.cleanupOldFiles(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to cleanup old files: %v\n", err)
		}
	}
	
	// Open new file
	if err := rfs.openFile(); err != nil {
		return err
	}
	
	// Update roll time
	rfs.updateRollTime()
	
	return nil
}

// updateRollTime calculates the next roll time based on interval.
func (rfs *RollingFileSink) updateRollTime() {
	now := time.Now()
	
	switch rfs.options.RollingInterval {
	case RollingIntervalHourly:
		rfs.rollTime = now.Truncate(time.Hour).Add(time.Hour)
	case RollingIntervalDaily:
		// Roll at midnight local time
		year, month, day := now.Date()
		rfs.rollTime = time.Date(year, month, day+1, 0, 0, 0, 0, now.Location())
	case RollingIntervalWeekly:
		// Roll on Sunday midnight local time
		days := int(time.Sunday - now.Weekday())
		if days <= 0 {
			days += 7
		}
		year, month, day := now.Date()
		rfs.rollTime = time.Date(year, month, day+days, 0, 0, 0, 0, now.Location())
	case RollingIntervalMonthly:
		// Roll on first day of next month
		year, month, _ := now.Date()
		rfs.rollTime = time.Date(year, month+1, 1, 0, 0, 0, 0, now.Location())
	default:
		rfs.rollTime = time.Now().Add(100 * 365 * 24 * time.Hour) // Far future
	}
}

// compressFile compresses a file using gzip.
func (rfs *RollingFileSink) compressFile(filePath string) error {
	// Read the source file completely first
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	
	destPath := filePath + ".gz"
	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	
	gz := gzip.NewWriter(dest)
	
	// Write compressed data
	if _, err := gz.Write(data); err != nil {
		gz.Close()
		dest.Close()
		os.Remove(destPath)
		return err
	}
	
	// Close gzip writer to flush
	if err := gz.Close(); err != nil {
		dest.Close()
		os.Remove(destPath)
		return err
	}
	
	// Close destination file
	if err := dest.Close(); err != nil {
		os.Remove(destPath)
		return err
	}
	
	// Remove original file - retry a few times on Windows
	for i := 0; i < 3; i++ {
		if err := os.Remove(filePath); err == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	
	return os.Remove(filePath)
}

// cleanupOldFiles removes old rolled files based on retention policy.
func (rfs *RollingFileSink) cleanupOldFiles() error {
	dir := filepath.Dir(rfs.options.FilePath)
	pattern := rfs.baseFileName + "-*" + rfs.fileExt
	if rfs.options.CompressRolledFiles {
		pattern += ".gz"
	}
	
	// Find all rolled files
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return err
	}
	
	// Sort by modification time (newest first)
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	
	files := make([]fileInfo, 0, len(matches))
	for _, match := range matches {
		stat, err := os.Stat(match)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{
			path:    match,
			modTime: stat.ModTime(),
		})
	}
	
	// Sort by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})
	
	// Remove old files
	for i := rfs.options.RetainFileCount; i < len(files); i++ {
		if err := os.Remove(files[i].path); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove old file %s: %v\n", files[i].path, err)
		}
	}
	
	return nil
}

// bufferedWriter wraps a writer with a buffer and provides thread-safe flushing.
type bufferedWriter struct {
	w   io.Writer
	buf []byte
	n   int
}

func newBufferedWriter(w io.Writer, size int) *bufferedWriter {
	return &bufferedWriter{
		w:   w,
		buf: make([]byte, size),
	}
}

func (bw *bufferedWriter) Write(p []byte) (int, error) {
	nn := 0
	for len(p) > 0 {
		n := copy(bw.buf[bw.n:], p)
		bw.n += n
		nn += n
		p = p[n:]
		
		if bw.n >= len(bw.buf) {
			if err := bw.Flush(); err != nil {
				return nn, err
			}
		}
	}
	return nn, nil
}

func (bw *bufferedWriter) Flush() error {
	if bw.n == 0 {
		return nil
	}
	
	n, err := bw.w.Write(bw.buf[:bw.n])
	if n < bw.n && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		if n > 0 && n < bw.n {
			copy(bw.buf[0:bw.n-n], bw.buf[n:bw.n])
		}
		bw.n -= n
		return err
	}
	bw.n = 0
	return nil
}