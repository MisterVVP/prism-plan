package api

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"prism-api/domain"
)

const walHeaderSize = 16

var (
	errWALClosed = errors.New("wal closed")
	crcTable     = crc32.MakeTable(crc32.Castagnoli)
)

type walConfig struct {
	dir          string
	segmentBytes int64
	syncEvery    int
	syncInterval time.Duration
	logger       *log.Logger
}

type walSegment struct {
	baseOffset uint64
	lastOffset uint64
	file       *os.File
	writer     *bufio.Writer
	size       int64
	path       string
}

type walRecord struct {
	Offset      uint64           `json:"offset"`
	UserID      string           `json:"userId"`
	Commands    []domain.Command `json:"commands"`
	AddedKeys   []string         `json:"addedKeys"`
	Timestamp   time.Time        `json:"timestamp"`
	Attempt     int              `json:"attempt"`
	LastErr     string           `json:"lastErr,omitempty"`
	encodedSize int64            `json:"-"`
}

type wal struct {
	cfg             walConfig
	mu              sync.Mutex
	segments        []*walSegment
	nextOffset      uint64
	committedOffset uint64
	closed          bool
	pendingSync     int
	lastSync        time.Time
	syncTimer       *time.Timer
}

func openWAL(cfg walConfig) (*wal, []*walRecord, error) {
	if cfg.dir == "" {
		return nil, nil, fmt.Errorf("wal dir required")
	}
	if err := os.MkdirAll(cfg.dir, 0o755); err != nil {
		return nil, nil, err
	}

	w := &wal{cfg: cfg}
	checkpoint, err := w.readCheckpoint()
	if err != nil {
		return nil, nil, err
	}
	w.committedOffset = checkpoint
	w.nextOffset = checkpoint + 1

	segments, err := filepath.Glob(filepath.Join(cfg.dir, "segment-*.wal"))
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(segments)

	pending := make([]*walRecord, 0)
	for _, path := range segments {
		seg, segRecords, err := w.loadSegment(path)
		if err != nil {
			return nil, nil, err
		}
		if seg == nil {
			continue
		}
		w.segments = append(w.segments, seg)
		for _, rec := range segRecords {
			if rec.Offset >= w.nextOffset {
				w.nextOffset = rec.Offset + 1
			}
			if rec.Offset > w.committedOffset {
				pending = append(pending, rec)
			}
		}
	}

	if len(w.segments) == 0 {
		if err := w.openNewSegmentLocked(); err != nil {
			return nil, nil, err
		}
	} else {
		// ensure writer on last segment is positioned correctly
		last := w.segments[len(w.segments)-1]
		if _, err := last.file.Seek(last.size, io.SeekStart); err != nil {
			return nil, nil, err
		}
		last.writer = bufio.NewWriterSize(last.file, 64*1024)
	}

	if cfg.syncInterval > 0 {
		w.syncTimer = time.NewTimer(cfg.syncInterval)
	}

	return w, pending, nil
}

func (w *wal) readCheckpoint() (uint64, error) {
	path := filepath.Join(w.cfg.dir, "checkpoint")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return 0, nil
	}
	val, err := strconv.ParseUint(trimmed, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid checkpoint: %w", err)
	}
	return val, nil
}

func (w *wal) loadSegment(path string) (*walSegment, []*walRecord, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}

	seg := &walSegment{path: path, file: f, size: fi.Size()}
	records := make([]*walRecord, 0)
	reader := bufio.NewReaderSize(f, 64*1024)
	var offset uint64
	var pos int64
	for {
		hdr := make([]byte, walHeaderSize)
		start := pos
		n, err := io.ReadFull(reader, hdr)
		pos += int64(n)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				// Truncate partial header
				if truncateErr := f.Truncate(start); truncateErr != nil {
					return nil, nil, truncateErr
				}
				break
			}
			return nil, nil, err
		}

		length := binary.LittleEndian.Uint32(hdr[0:4])
		crc := binary.LittleEndian.Uint32(hdr[4:8])
		recOffset := binary.LittleEndian.Uint64(hdr[8:16])
		if length == 0 {
			continue
		}
		buf := make([]byte, length)
		n, err = io.ReadFull(reader, buf)
		pos += int64(n)
		if err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
				if truncateErr := f.Truncate(start); truncateErr != nil {
					return nil, nil, truncateErr
				}
				break
			}
			return nil, nil, err
		}

		if crc32.Checksum(buf, crcTable) != crc {
			if err := f.Truncate(start); err != nil {
				return nil, nil, err
			}
			break
		}

		var rec walRecord
		if err := jsonUnmarshal(buf, &rec); err != nil {
			return nil, nil, err
		}
		if rec.Offset != recOffset {
			return nil, nil, fmt.Errorf("wal offset mismatch: header=%d payload=%d", recOffset, rec.Offset)
		}
		if seg.baseOffset == 0 {
			seg.baseOffset = rec.Offset
		}
		seg.lastOffset = rec.Offset
		rec.encodedSize = int64(walHeaderSize) + int64(length)
		offset = rec.Offset
		records = append(records, &rec)
	}

	seg.size = pos
	if seg.baseOffset == 0 {
		seg.baseOffset = offset
	}

	return seg, records, nil
}

func (w *wal) openNewSegmentLocked() error {
	if w.closed {
		return errWALClosed
	}
	name := fmt.Sprintf("segment-%020d.wal", w.nextOffset)
	path := filepath.Join(w.cfg.dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	seg := &walSegment{
		baseOffset: w.nextOffset,
		lastOffset: w.nextOffset - 1,
		file:       f,
		writer:     bufio.NewWriterSize(f, 64*1024),
		size:       0,
		path:       path,
	}
	w.segments = append(w.segments, seg)
	return nil
}

func (w *wal) appendRecordLocked(rec *walRecord) error {
	if w.closed {
		return errWALClosed
	}
	if len(w.segments) == 0 {
		if err := w.openNewSegmentLocked(); err != nil {
			return err
		}
	}
	current := w.segments[len(w.segments)-1]
	if current.size >= w.cfg.segmentBytes {
		if err := current.writer.Flush(); err != nil {
			return err
		}
		if err := current.file.Sync(); err != nil {
			return err
		}
		current.writer = nil
		if err := current.file.Close(); err != nil {
			return err
		}
		if err := w.openNewSegmentLocked(); err != nil {
			return err
		}
		current = w.segments[len(w.segments)-1]
	}

	rec.Offset = w.nextOffset
	w.nextOffset++

	payload, err := jsonMarshal(rec)
	if err != nil {
		return err
	}
	header := make([]byte, walHeaderSize)
	binary.LittleEndian.PutUint32(header[0:4], uint32(len(payload)))
	binary.LittleEndian.PutUint32(header[4:8], crc32.Checksum(payload, crcTable))
	binary.LittleEndian.PutUint64(header[8:16], rec.Offset)

	if _, err := current.writer.Write(header); err != nil {
		return err
	}
	if _, err := current.writer.Write(payload); err != nil {
		return err
	}
	if err := current.writer.Flush(); err != nil {
		return err
	}

	rec.encodedSize = int64(len(header) + len(payload))
	current.size += rec.encodedSize
	current.lastOffset = rec.Offset
	w.pendingSync++
	return nil
}

func (w *wal) rollbackRecordLocked(rec *walRecord) error {
	if len(w.segments) == 0 {
		return nil
	}
	current := w.segments[len(w.segments)-1]
	if rec.Offset != current.lastOffset {
		return fmt.Errorf("rollback mismatch: offset=%d last=%d", rec.Offset, current.lastOffset)
	}
	delta := rec.encodedSize
	if current.size < delta {
		return fmt.Errorf("rollback underflow")
	}
	current.size -= delta
	if err := current.file.Truncate(current.size); err != nil {
		return err
	}
	if _, err := current.file.Seek(current.size, io.SeekStart); err != nil {
		return err
	}
	current.writer = bufio.NewWriterSize(current.file, 64*1024)
	w.nextOffset = rec.Offset
	current.lastOffset--
	return nil
}

func (w *wal) syncIfNeededLocked() error {
	if w.cfg.syncEvery <= 1 {
		return w.syncLocked()
	}
	if w.pendingSync >= w.cfg.syncEvery {
		return w.syncLocked()
	}
	if w.cfg.syncInterval <= 0 {
		return nil
	}
	if w.lastSync.IsZero() {
		w.lastSync = time.Now()
	}
	return nil
}

func (w *wal) syncLocked() error {
	if w.closed {
		return errWALClosed
	}
	if len(w.segments) == 0 {
		return nil
	}
	current := w.segments[len(w.segments)-1]
	if current.writer != nil {
		if err := current.writer.Flush(); err != nil {
			return err
		}
	}
	if err := current.file.Sync(); err != nil {
		return err
	}
	w.pendingSync = 0
	w.lastSync = time.Now()
	return nil
}

func (w *wal) commitLocked(offset uint64) error {
	if offset <= w.committedOffset {
		return nil
	}
	w.committedOffset = offset
	path := filepath.Join(w.cfg.dir, "checkpoint")
	tmp := path + ".tmp"
	data := []byte(strconv.FormatUint(offset, 10))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := syncFile(tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := syncDir(w.cfg.dir); err != nil {
		return err
	}
	w.pruneSegmentsLocked()
	return nil
}

func (w *wal) pruneSegmentsLocked() {
	for len(w.segments) > 1 {
		seg := w.segments[0]
		if seg.lastOffset > w.committedOffset {
			break
		}
		if seg.writer != nil {
			seg.writer.Flush()
		}
		seg.file.Close()
		if err := os.Remove(seg.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			if w.cfg.logger != nil {
				w.cfg.logger.WithError(err).Warnf("failed to remove wal segment %s", seg.path)
			}
			break
		}
		w.segments = w.segments[1:]
	}
}

func (w *wal) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	if w.syncTimer != nil {
		w.syncTimer.Stop()
	}
	for _, seg := range w.segments {
		if seg.writer != nil {
			seg.writer.Flush()
		}
		seg.file.Close()
	}
	return nil
}

func syncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
