package codec

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/fxamacker/cbor/v2"
)

type Codec interface {
	Marshal(any) ([]byte, error)
	Unmarshal([]byte, any) error
}

func New(name string) (Codec, error) {
	switch name {
	case "json":
		return jsonCodec{}, nil
	case "cbor":
		return newCBORCodec()
	default:
		return nil, fmt.Errorf("unknown encoding: %q", name)
	}
}

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

type cborCodec struct {
	enc cbor.EncMode
	dec cbor.DecMode
}

func newCBORCodec() (cborCodec, error) {
	encOpts := cbor.CanonicalEncOptions()
	enc, err := encOpts.EncMode()
	if err != nil {
		return cborCodec{}, fmt.Errorf("cbor enc mode: %w", err)
	}

	decOpts := cbor.DecOptions{
		DefaultMapType: reflect.TypeFor[map[string]any](),
	}
	dec, err := decOpts.DecMode()
	if err != nil {
		return cborCodec{}, fmt.Errorf("cbor dec mode: %w", err)
	}

	return cborCodec{enc: enc, dec: dec}, nil
}

func (c cborCodec) Marshal(v any) ([]byte, error) {
	return c.enc.Marshal(v)
}

func (c cborCodec) Unmarshal(data []byte, v any) error {
	return c.dec.Unmarshal(data, v)
}
