package session

import (
	"fmt"
	"io"
	"os"
	"time"
)

func (g *Global) Log(msg string, args ...any) {
	g.log(os.Stderr, true, msg, args...)
}

func (s *Regional) Log(msg string, args ...any) {
	s.Global.log(os.Stderr, true, fmt.Sprintf("%s: %s", s.Region, msg), args...)
}

func (s *Regional) Print(msg string, args ...any) {
	s.Global.log(os.Stdout, false, msg, args...)
}

func (g *Global) log(w io.Writer, withTime bool, msg string, args ...any) {
	msg = fmt.Sprintf(msg, args...)
	if withTime {
		msg = fmt.Sprintf("[%s] %s", time.Now().Format(datetimeFormat), msg)
	}

	switch l := len(msg); l {
	case 0:
		return
	default:
		printFn := fmt.Fprint
		if msg[l-1] != '\n' {
			printFn = fmt.Fprintln
		}

		g.logMutex.Lock()
		defer g.logMutex.Unlock()

		printFn(w, msg)
	}
}

const (
	datetimeFormat = "2006-01-02 15:04:05"
)
