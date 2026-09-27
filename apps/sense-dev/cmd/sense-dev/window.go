package main

import (
	"errors"
	"strings"
	"time"
)

type runWindow struct {
	start, end int
	location   *time.Location
}

func parseRunWindow(value string) (runWindow, error) {
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return runWindow{}, errors.New("inference window must be HH:MM-HH:MM in JST")
	}
	start, err := time.Parse("15:04", parts[0])
	if err != nil {
		return runWindow{}, errors.New("invalid inference window start")
	}
	end, err := time.Parse("15:04", parts[1])
	if err != nil {
		return runWindow{}, errors.New("invalid inference window end")
	}
	startMinute, endMinute := start.Hour()*60+start.Minute(), end.Hour()*60+end.Minute()
	if startMinute == endMinute {
		return runWindow{}, errors.New("inference window cannot cover an unspecified full day")
	}
	location, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		return runWindow{}, err
	}
	return runWindow{start: startMinute, end: endMinute, location: location}, nil
}

func (w runWindow) contains(now time.Time) bool {
	local := now.In(w.location)
	minute := local.Hour()*60 + local.Minute()
	if w.start < w.end {
		return minute >= w.start && minute < w.end
	}
	return minute >= w.start || minute < w.end
}
