package model

func ValidateDeviceProfile(messageProtocol, payloadEncoding string) error {
	if messageProtocol != "" && messageProtocol != "legacy" && messageProtocol != "simple" {
		return MkHttpErr("messageProtocol must be legacy or simple", 400, 400)
	}
	if payloadEncoding != "" && payloadEncoding != "json" && payloadEncoding != "cbor" {
		return MkHttpErr("payloadEncoding must be json or cbor", 400, 400)
	}
	return nil
}
