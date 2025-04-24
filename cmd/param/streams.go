package param

import (
	"bytes"
	"io"
	"os"
)

// Creates memory-based file for testing logging
func CreateStream() (*os.File, chan string) {
	r, outStream, _ := os.Pipe()

	outC := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outC <- buf.String()
	}()

	return outStream, outC
}

// Reads from memory-based file for testing logging
func ReadStream(outStream *os.File, outC chan string) string {
	outStream.Close()

	output := <-outC
	return output
}
