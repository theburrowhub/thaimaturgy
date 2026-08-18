package dnd5e

import (
	"github.com/theburrowhub/thaimaturgy/internal/jsonstrict"
	core "github.com/theburrowhub/thaimaturgy/internal/rules"
)

func decodePayload(payload core.Payload, dst any) error {
	return jsonstrict.Decode(payload.Bytes(), dst)
}

func payloadFrom(value any) (core.Payload, error) {
	return core.PayloadFrom(value)
}
