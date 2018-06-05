package avro

import "github.com/MemeLabs/overrustlelogs/common"

// NewMessageFromCommonMessage ...
func NewMessageFromCommonMessage(channel string, m *common.Message) *Message {
	return &Message{
		Time:    m.Time.Unix(),
		Channel: channel,
		Nick:    m.Nick,
		Message: m.Data,
	}
}
