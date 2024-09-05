package model

type StatusInfo struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
	Error  error  `json:"error"`
	Metric any    `json:"metric"`
}

const (
	Connected    = "connected"
	Disconnected = "disconnected"
)

func StatusNotStarted() StatusInfo {
	return StatusInfo{
		Status: Disconnected,
		Reason: "not started",
	}
}

func StatusConnected() StatusInfo {
	return StatusInfo{
		Status: Connected,
	}
}

func StatusConnecting() StatusInfo {
	return StatusInfo{
		Status: Disconnected,
		Reason: "connecting",
	}
}

func StatusDisconnected(reason string, err error) StatusInfo {
	return StatusInfo{
		Status: Disconnected,
		Reason: reason,
		Error:  err,
	}
}
