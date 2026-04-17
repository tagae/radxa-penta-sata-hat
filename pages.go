package main

import (
	"fmt"

	"radxa-penta/display"
	"radxa-penta/sys"
)

func buildPages(cfg *Config) []display.Page {
	return []display.Page{
		{
			{0, 0, sys.GetUptime()},
			{0, 11, sys.GetCPUTemp(cfg.OLED.FTemp)},
			{0, 21, sys.GetIP()},
		},
		{
			{0, 2, sys.GetCPULoad()},
			{0, 18, sys.GetMemory()},
		},
		diskUsagePage(cfg),
		diskTempPage(cfg),
	}
}

func diskUsagePage(cfg *Config) display.Page {
	keys, vals := sys.GetDiskUsage(cfg.GetDisks())
	if len(keys) == 0 {
		return display.Page{{0, 10, "Disk: N/A"}}
	}

	text1 := fmt.Sprintf("Disk: %s %s", keys[0], vals[0])

	switch len(keys) {
	case 5:
		text2 := fmt.Sprintf("%s %s  %s %s", keys[1], vals[1], keys[2], vals[2])
		text3 := fmt.Sprintf("%s %s  %s %s", keys[3], vals[3], keys[4], vals[4])
		return display.Page{
			{0, 0, text1},
			{0, 11, text2},
			{0, 21, text3},
		}
	case 3:
		text2 := fmt.Sprintf("%s %s  %s %s", keys[1], vals[1], keys[2], vals[2])
		return display.Page{
			{0, 2, text1},
			{0, 18, text2},
		}
	case 2:
		return display.Page{
			{0, 2, text1},
			{0, 18, fmt.Sprintf("%s %s", keys[1], vals[1])},
		}
	default:
		return display.Page{{0, 10, text1}}
	}
}

func diskTempPage(cfg *Config) display.Page {
	keys, vals := sys.GetDiskTemps(cfg.GetDisks())
	if len(keys) == 0 {
		return display.Page{{0, 10, "Temp: N/A"}}
	}

	n := len(keys)
	if n > 6 {
		n = 6
	}
	keys, vals = keys[:n], vals[:n]

	pair := func(i int) string {
		if i+1 < n {
			return fmt.Sprintf("%s %s  %s %s", keys[i], vals[i], keys[i+1], vals[i+1])
		}
		return fmt.Sprintf("%s %s", keys[i], vals[i])
	}

	switch {
	case n == 1:
		return display.Page{{0, 10, pair(0)}}
	case n == 2:
		return display.Page{
			{0, 2, pair(0)},
			{0, 18, pair(1)},
		}
	case n <= 4:
		return display.Page{
			{0, 2, pair(0)},
			{0, 18, pair(2)},
		}
	default:
		return display.Page{
			{0, 0, pair(0)},
			{0, 11, pair(2)},
			{0, 21, pair(4)},
		}
	}
}
