package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

// readKey monitors the button GPIO and returns when a press pattern
// (click, twice, or press) is recognised.
func readKey(chipName string, lineNum int, patterns map[string]*regexp.Regexp, size int) string {
	line, err := gpiocdev.RequestLine(chipName, lineNum, gpiocdev.AsOutput(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "button GPIO %s/%d: %v\n", chipName, lineNum, err)
		time.Sleep(time.Second)
		return "none"
	}
	defer line.Close()

	s := ""
	for {
		v, _ := line.Value()
		s += strconv.Itoa(v)
		if len(s) > size {
			s = s[len(s)-size:]
		}

		for name, pat := range patterns {
			if pat.MatchString(s) {
				return name
			}
		}

		time.Sleep(100 * time.Millisecond)
	}
}

// WatchKey continuously monitors the hardware button and sends
// recognised events ("click", "twice", "press") to ch.
func WatchKey(cfg *Config, ch chan<- string) {
	size := int(cfg.Time.Press * 10)
	wait := int(cfg.Time.Twice * 10)

	patterns := map[string]*regexp.Regexp{
		"click": regexp.MustCompile(fmt.Sprintf(`1+0+1{%d,}`, wait)),
		"twice": regexp.MustCompile(`1+0+1+0+1{3,}`),
		"press": regexp.MustCompile(fmt.Sprintf(`1+0{%d,}`, size)),
	}

	chipName := ChipName(os.Getenv("BUTTON_CHIP"))
	lineNum, _ := strconv.Atoi(os.Getenv("BUTTON_LINE"))

	for {
		key := readKey(chipName, lineNum, patterns, size)
		ch <- key
	}
}
