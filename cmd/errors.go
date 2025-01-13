package cmd

import (
	"fmt"
	"os"
)

// NoBellStdoutWriter ...
type NoBellStdoutWriter struct{}

// NewNoBellStdoutWriter ...
func NewNoBellStdoutWriter() *NoBellStdoutWriter {
	return &NoBellStdoutWriter{}
}

func (w *NoBellStdoutWriter) Write(b []byte) (int, error) {
	if len(b) > 0 && b[0] == 7 {
		return 0, nil
	}
	return os.Stdout.Write(b)
}

// Close implements io.Closer interface
func (w *NoBellStdoutWriter) Close() error {
	return nil
}

func checkError(err error) {
	if err != nil {
		if sp != nil {
			sp.Stop()
		}
		exitWithMsg(err.Error())
	}
}

func exitWithMsg(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
