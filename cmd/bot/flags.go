package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pborzenkov/tg-bot-transmission/pkg/bot"
)

type locationsValue []bot.Location

func newLocationsValue(p *[]bot.Location) *locationsValue {
	return (*locationsValue)(p)
}

func (l *locationsValue) Set(s string) error {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return errors.New("invalid location value")
	}

	*l = append(*l, bot.Location{
		Name: parts[0],
		Path: parts[1],
	})

	return nil
}

func (l *locationsValue) String() string {
	locs := make([]string, 0, len(*l))
	for _, ll := range *l {
		locs = append(locs, fmt.Sprintf("%s:%s", ll.Name, ll.Path))
	}

	return strings.Join(locs, ",")
}
