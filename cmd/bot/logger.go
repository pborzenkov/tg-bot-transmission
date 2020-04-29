package main

import (
	"io"
	"log"
)

type logger struct {
	log     *log.Logger
	verbose bool
}

func newLogger(out io.Writer, verbose bool) *logger {
	return &logger{
		log:     log.New(out, "", log.Ldate|log.Ltime),
		verbose: verbose,
	}
}

func (l *logger) Infof(fmt string, args ...interface{}) {
	l.log.Printf(fmt, args...)
}

func (l *logger) Debugf(fmt string, args ...interface{}) {
	if l.verbose {
		l.log.Printf(fmt, args...)
	}
}
