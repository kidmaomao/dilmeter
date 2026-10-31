package packet

import (
	"github.com/gopacket/gopacket/pcap"
	"time"
)

type captureLatencyOptions interface {
	SetSnapLen(int) error
	SetPromisc(bool) error
	SetTimeout(time.Duration) error
	SetBufferSize(int) error
	SetImmediateMode(bool) error
}

func configureImmediateCapture(h captureLatencyOptions, timeout time.Duration) error {
	if err := h.SetSnapLen(pcapBufferSize); err != nil {
		return err
	}
	if err := h.SetPromisc(pcapPromisc); err != nil {
		return err
	}
	if err := h.SetTimeout(timeout); err != nil {
		return err
	}
	if err := h.SetBufferSize(pcapBufferSize); err != nil {
		return err
	}
	return h.SetImmediateMode(true)
}

func openLowLatencyCapture(name string, timeout time.Duration) (*pcap.Handle, error) {
	inactive, err := pcap.NewInactiveHandle(name)
	if err == nil {
		defer inactive.CleanUp()
		if err = configureImmediateCapture(inactive, timeout); err == nil {
			var h *pcap.Handle
			h, err = inactive.Activate()
			if err == nil {
				logger.Println("pcap immediate delivery enabled")
				return h, nil
			}
		}
	}
	// Retain compatibility with older capture drivers that lack immediate mode.
	logger.Printf("pcap immediate mode unavailable (%v); using compatible capture", err)
	return pcap.OpenLive(name, pcapBufferSize, pcapPromisc, timeout)
}
