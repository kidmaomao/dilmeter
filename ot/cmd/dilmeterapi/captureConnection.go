package main

import "time"

const captureDisconnectGrace = 8 * time.Second
const captureDisconnectChecks = 3

type captureConnectionMonitor struct {
	lastPayloadCount uint64
	lastPayloadAt    time.Time
	missingChecks    int
	lastError        string
	lastErrorLogAt   time.Time
}

func (m *captureConnectionMonitor) reportError(now time.Time, err error) {
	if err != nil {
		if err.Error() != m.lastError || now.Sub(m.lastErrorLogAt) >= 30*time.Second {
			logger.Printf("connection status uncertain; retaining capture state: %v", err)
			m.lastErrorLogAt = now
		}
		m.lastError = err.Error()
		return
	}
	if m.lastError != "" {
		logger.Println("connection status query recovered")
		m.lastError = ""
	}
}

func (m *captureConnectionMonitor) shouldClose(now time.Time, payloadCount uint64, connected bool, err error) bool {
	if m.lastPayloadAt.IsZero() || payloadCount != m.lastPayloadCount {
		m.lastPayloadAt = now
		m.lastPayloadCount = payloadCount
	}
	// Query errors are unknown results, not evidence of a disconnect.
	if err != nil || connected {
		if connected && m.missingChecks > 0 {
			logger.Printf("capture connection recovered after %d missing checks", m.missingChecks)
		}
		m.missingChecks = 0
		return false
	}
	if m.missingChecks == 0 {
		logger.Printf("capture connection absent from TCP snapshot; verifying before close (payloads=%d idle=%s)",
			payloadCount, now.Sub(m.lastPayloadAt).Round(time.Millisecond))
	}
	if m.missingChecks < captureDisconnectChecks {
		m.missingChecks++
	}
	return m.missingChecks >= captureDisconnectChecks && now.Sub(m.lastPayloadAt) >= captureDisconnectGrace
}
