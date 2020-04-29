package main

import (
	"strings"
	"testing"
)

func TestLogger(t *testing.T) {
	out := new(strings.Builder)

	l := newLogger(out, false)
	l.Infof("info message")
	l.Debugf("debug message")

	if !strings.Contains(out.String(), "info message") {
		t.Errorf("expected info message to make it")
	}
	if strings.Contains(out.String(), "debug message") {
		t.Errorf("didn't expect debug message to make it")
	}

	out.Reset()

	l = newLogger(out, true)
	l.Infof("info message")
	l.Debugf("debug message")

	if !strings.Contains(out.String(), "info message") {
		t.Errorf("expected info message to make it")
	}
	if !strings.Contains(out.String(), "debug message") {
		t.Errorf("expected debug message to make it")
	}
}
