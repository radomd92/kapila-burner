package core

import "strconv"

type Recorder struct {
	Device string `json:"device"`
	Name   string `json:"name"`
}

type BurnRequest struct {
	JobID     string `json:"jobId"`
	Device    string `json:"device"`
	Speed     int    `json:"speed"`
	Dummy     bool   `json:"dummy"`
	Confirmed bool   `json:"confirmed"`
}

func burnArgs(request BurnRequest, cue string) []string {
	args := []string{"write", "--device", request.Device, "--speed", strconv.Itoa(request.Speed)}
	if request.Dummy {
		args = append(args, "--simulate")
	}
	return append(args, cue)
}
