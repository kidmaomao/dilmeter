package packet

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/pcapgo"
	"gitlab.com/prilus/mabidilmeter/constants"
)

// Each receive loop owns its log reference. Closing/reopening the reader cannot
// redirect late writes from the old connection into the next capture file.
type captureLog struct {
	mu          sync.Mutex
	file        *os.File
	writer      *pcapgo.NgWriter
	writeFailed bool
}

func (t *GameServerPacketReader) openLog() error {
	if err := os.MkdirAll(t.logDir, os.ModePerm); err != nil {
		return err
	}
	for {
		name := fmt.Sprintf("packet_capture_%d.pcapng", constants.SERVER_START_AT)
		if t.captureLogIndex > 0 {
			name = fmt.Sprintf("packet_capture_%d_%03d.pcapng", constants.SERVER_START_AT, t.captureLogIndex)
		}
		t.captureLogIndex++
		path := filepath.Join(t.logDir, name)
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		writer, err := pcapgo.NewNgWriter(file, t.linkType)
		if err == nil {
			err = writer.Flush()
		}
		if err != nil {
			file.Close()
			return fmt.Errorf("initialize capture log %q: %w", path, err)
		}
		t.captureLog = &captureLog{file: file, writer: writer}
		logger.Printf("pcapng writer initialized: path=%q LinkType=%v", path, t.linkType)
		return nil
	}
}

func (l *captureLog) write(ci gopacket.CaptureInfo, data []byte) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil || l.writeFailed {
		return
	}
	if err := l.writer.WritePacket(ci, data); err != nil {
		l.writeFailed = true
		logger.Printf("capture log write failed: path=%q: %v", l.file.Name(), err)
	}
}

func (l *captureLog) close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return
	}
	if err := l.writer.Flush(); err != nil {
		logger.Printf("capture log flush failed: path=%q: %v", l.file.Name(), err)
	}
	if err := l.file.Close(); err != nil {
		logger.Printf("capture log close failed: path=%q: %v", l.file.Name(), err)
	}
	l.file = nil
}

// IsCaptureLogForSession recognizes both legacy files and reconnect segments.
func IsCaptureLogForSession(name string, startedAt int64) bool {
	name = strings.ToLower(name)
	base := fmt.Sprintf("packet_capture_%d", startedAt)
	if name == base+".pcapng" {
		return true
	}
	if !strings.HasPrefix(name, base+"_") || !strings.HasSuffix(name, ".pcapng") {
		return false
	}
	part := strings.TrimSuffix(strings.TrimPrefix(name, base+"_"), ".pcapng")
	index, err := strconv.ParseUint(part, 10, 64)
	return err == nil && index > 0
}
