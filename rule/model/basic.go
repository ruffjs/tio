package model

type StatusInfo struct {
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Error   error  `json:"error"`
	Stopped bool   `json:"stopped"`
}

const (
	Connected    = "connected"
	Disconnected = "disconnected"
)

func StatusNotStarted() StatusInfo {
	return StatusInfo{
		Status:  Disconnected,
		Reason:  "not started",
		Stopped: true,
	}
}

func StatusConnected() StatusInfo {
	return StatusInfo{
		Status: Connected,
	}
}

func StatusDisconnected(reason string, err error) StatusInfo {
	return StatusInfo{
		Status: Disconnected,
		Reason: reason,
		Error:  err,
	}
}
