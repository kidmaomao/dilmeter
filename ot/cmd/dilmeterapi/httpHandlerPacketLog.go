package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func httpHandlerPacketLog(w http.ResponseWriter, r *http.Request) {
	fd, err := os.Open(filepath.Join(_logDir, packetLogFilename))
	if err != nil {
		http.Error(w, err.Error(), 404)
		logger.Println("Error opening packet log file:", err.Error())

		return
	}

	defer fd.Close()

	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", packetLogFilename))

	http.ServeContent(w, r, packetLogFilename, time.Now(), fd)
}
