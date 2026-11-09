package connector

import "context"

type DeviceProfile struct {
	MessageProtocol string `json:"messageProtocol"`
	PayloadEncoding string `json:"payloadEncoding"`
}

type DeviceProfileGetter interface {
	GetDeviceProfile(ctx context.Context, thingId string) (DeviceProfile, error)
}
