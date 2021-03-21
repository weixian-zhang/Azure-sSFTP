
package logc

import (
	"os"
	"log"
)

type StdLogClient struct {
	stdoutWriter *log.Logger
	stderrWriter *log.Logger
}

func NewStdLogClient() (StdLogClient) {
	stdoutWriter := log.New(os.Stdout, "", 0)
	stderrWriter := log.New(os.Stdout, "", 0)

	return StdLogClient {
		stdoutWriter: stdoutWriter,
		stderrWriter: stderrWriter,
	}
}

func (stdl StdLogClient) Info(msg string) {
	stdl.stdoutWriter.Println(msg)
}

func (stdl StdLogClient) Err(err error) {
	stdl.stderrWriter.Println(err.Error())
}

