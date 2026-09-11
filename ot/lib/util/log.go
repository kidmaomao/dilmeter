package util

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
)

var writer = &switchableWriter{}

func init() {
	w := io.Writer(os.Stdout)
	writer.w.Store(&w)
}

var _ io.Writer = (*switchableWriter)(nil)

type switchableWriter struct {
	w atomic.Pointer[io.Writer]
}

func (sw *switchableWriter) Write(p []byte) (n int, err error) {
	return (*sw.w.Load()).Write(p)
}

func LogInit(logFileName string) error {
	if err := os.MkdirAll(filepath.Dir(logFileName), os.ModePerm); err != nil {
		return err
	}

	f, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	// GUI builds do not have a usable stdout handle. Write to the file first so
	// an stdout error cannot prevent diagnostics from reaching disk.
	w := io.MultiWriter(f, os.Stdout)
	writer.w.Store(&w)
	return nil
}

func NewLogger(name string) *log.Logger {
	return log.New(writer, name+" ", log.LstdFlags|log.Lshortfile)
}
