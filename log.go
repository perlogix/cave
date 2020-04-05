package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo"
)

// Log type
type Log struct {
	FormatString string
	c            chan string
	terminator   chan bool
}

// New logger
func (l Log) New(config *Config) *Log {
	log := &Log{
		FormatString: "%s [ %-5s ] %v\n",
		c:            make(chan string, config.Perf.BufferSize),
		terminator:   make(chan bool),
	}
	return log
}

//Start function
func (l *Log) Start() {
	for {
		select {
		case <-l.terminator:
			// Finish writing logs before quitting
			for m := range l.c {
				fmt.Printf(m)
			}
			return
		case m := <-l.c:
			fmt.Printf(m)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05.000 MST")
}

func (l *Log) print(lvl string, msg string) {
	l.c <- fmt.Sprintf(l.FormatString, timestamp(), lvl, msg)
}

// Debug method
func (l *Log) Debug(v ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		l.print("DEBUG", fmt.Sprint(v...))
	}
}

// DebugF func
func (l *Log) DebugF(s string, v ...interface{}) {
	if os.Getenv("DEBUG") != "" {
		l.print("DEBUG", fmt.Sprintf(s, v...))
	}
}

// Info method
func (l *Log) Info(v ...interface{}) {
	l.print("INFO", fmt.Sprint(v...))
}

// InfoF func
func (l *Log) InfoF(s string, v ...interface{}) {
	l.print("INFO", fmt.Sprintf(s, v...))
}

//Warn func
func (l *Log) Warn(v ...interface{}) {
	l.print("WARN", fmt.Sprint(v...))
}

//WarnF func
func (l *Log) WarnF(s string, v ...interface{}) {
	l.print("WARN", fmt.Sprintf(s, v...))
}

//Error func
func (l *Log) Error(v ...interface{}) {
	l.print("ERROR", fmt.Sprint(v...))
}

//ErrorF func
func (l *Log) ErrorF(s string, v ...interface{}) {
	l.print("ERROR", fmt.Sprintf(s, v...))
}

//Fatal func
func (l *Log) Fatal(v ...interface{}) {
	l.print("FATAL", fmt.Sprint(v...))
	os.Exit(1)
}

//FatalF func
func (l *Log) FatalF(s string, v ...interface{}) {
	l.print("FATAL", fmt.Sprintf(s, v...))
	os.Exit(1)
}

//Panic func
func (l *Log) Panic(v ...interface{}) {
	panic(fmt.Sprint(v...))
}

//PanicF func
func (l *Log) PanicF(s string, v ...interface{}) {
	panic(fmt.Sprintf(s, v...))
}

// Pretty log
func (l *Log) Pretty(v ...interface{}) {
	for _, i := range v {
		j, _ := json.MarshalIndent(i, "", "  ")
		if string(j[:]) != "null" {
			l.print("PRETTY", "\n"+string(j[:]))
		}
	}
}

// Middleware is an echo logger middleware
func (l *Log) middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().After(func() {
			l.print(strings.ToUpper(c.Scheme()), fmt.Sprintf(
				"%3v %-7s %s",
				c.Response().Status,
				c.Request().Method,
				c.Request().RequestURI,
			))
		})
		return next(c)
	}
}

//EchoLogger logger
func (l *Log) EchoLogger() echo.MiddlewareFunc {
	return l.middleware
}
