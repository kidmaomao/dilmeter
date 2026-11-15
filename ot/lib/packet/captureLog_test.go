package packet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
	"gitlab.com/prilus/mabidilmeter/constants"
)

func TestCaptureLogsRetainReconnectsAndExistingFiles(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, fmt.Sprintf("packet_capture_%d.pcapng", constants.SERVER_START_AT))
	if err := os.WriteFile(base, []byte("existing evidence"), 0644); err != nil {
		t.Fatal(err)
	}
	r := &GameServerPacketReader{ctx: context.Background(), logDir: dir}
	var previous *captureLog
	for index, linkType := range []layers.LinkType{layers.LinkTypeEthernet, layers.LinkTypeIPv4, layers.LinkTypeNull} {
		r.linkType = linkType
		if err := r.openLog(); err != nil {
			t.Fatal(err)
		}
		path := r.captureLog.file.Name()
		data := []byte{byte(index), 1, 2, 3}
		ci := gopacket.CaptureInfo{Timestamp: time.Now(), CaptureLength: len(data), Length: len(data)}
		// A delayed writer from the closed session cannot contaminate the new log.
		previous.write(ci, []byte{9, 9, 9, 9})
		r.captureLog.write(ci, data)
		previous = r.captureLog
		r.CloseNic()
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		reader, err := pcapgo.NewNgReader(file, pcapgo.DefaultNgReaderOptions)
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		got, _, err := reader.ReadPacketData()
		if err != nil || !bytes.Equal(got, data) || reader.LinkType() != linkType {
			file.Close()
			t.Fatalf("segment %d corrupted: bytes=%x link=%v err=%v", index, got, reader.LinkType(), err)
		}
		_, _, err = reader.ReadPacketData()
		file.Close()
		if err != io.EOF {
			t.Fatalf("extra data or incomplete pcapng block: %v", err)
		}
	}
	got, err := os.ReadFile(base)
	if err != nil || string(got) != "existing evidence" {
		t.Fatalf("existing capture overwritten: %q %v", got, err)
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.pcapng"))
	if err != nil || len(files) != 4 {
		t.Fatalf("reconnect segments missing: %v %v", files, err)
	}
}

func TestCaptureLogsConcurrentReadersUseDifferentFiles(t *testing.T) {
	dir := t.TempDir()
	var writers sync.WaitGroup
	for i := 0; i < 8; i++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			r := &GameServerPacketReader{logDir: dir, linkType: layers.LinkTypeEthernet}
			if err := r.openLog(); err != nil {
				t.Error(err)
				return
			}
			defer r.CloseNic()
			data := []byte{1, 2, 3, 4}
			ci := gopacket.CaptureInfo{Timestamp: time.Now(), CaptureLength: len(data), Length: len(data)}
			for n := 0; n < 20; n++ {
				r.captureLog.write(ci, data)
			}
		}()
	}
	writers.Wait()
	files, err := filepath.Glob(filepath.Join(dir, "*.pcapng"))
	if err != nil || len(files) != 8 {
		t.Fatalf("capture file collision: %v %v", files, err)
	}
	for _, path := range files {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		reader, err := pcapgo.NewNgReader(file, pcapgo.DefaultNgReaderOptions)
		if err != nil {
			file.Close()
			t.Fatal(err)
		}
		count := 0
		for {
			_, _, err = reader.ReadPacketData()
			if err != nil {
				break
			}
			count++
		}
		file.Close()
		if err != io.EOF || count != 20 {
			t.Fatalf("invalid saved segment: count=%d err=%v", count, err)
		}
	}
}

func TestCaptureLogSessionMatching(t *testing.T) {
	for _, name := range []string{"packet_capture_123.pcapng", "packet_capture_123_001.pcapng", "packet_capture_123_1000.pcapng"} {
		if !IsCaptureLogForSession(name, 123) {
			t.Errorf("segment not associated with session: %s", name)
		}
	}
	for _, name := range []string{"packet_capture_1234_001.pcapng", "packet_capture_123_backup.pcapng", "../packet_capture_123.pcapng", "packet_capture_123_001.txt"} {
		if IsCaptureLogForSession(name, 123) {
			t.Errorf("unrelated file associated with session: %s", name)
		}
	}
}
