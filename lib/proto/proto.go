package proto

import (
	"errors"
	"fmt"
	"strings"
)

type MessageType byte

type Message []byte

const (
	// Newline representation in hex.
	ByteLF = byte(0x0A)

	MsgRelayHello   = MessageType(0x01)
	MsgRelayConfig  = MessageType(0x02)
	MsgGatewayProxy = MessageType(0x03)

	MsgSuccess = MessageType(0xFE)
	MsgError   = MessageType(0xFF)

	KingIP = "king-ip"
)

var ErrorMessageTypeUnknown = errors.New("Unknown message type")
var ErrorMessageTypeNotMap = errors.New("Invalid message type for map")

func (m MessageType) Validate() error {
	switch m {
	case MsgRelayHello, MsgRelayConfig, MsgGatewayProxy, MsgSuccess, MsgError:
		return nil
	default:
		return ErrorMessageTypeUnknown
	}
}

func (m Message) Type() (MessageType, error) {
	mt := MessageType(m[0])
	return mt, mt.Validate()
}

func (m Message) UnmarshalString() (MessageType, string, error) {
	mt := MessageType(m[0])

	// If a message is shorter than two bytes then it has no body.
	if len(m) < 2 {
		return mt, "", mt.Validate()
	}

	// Removing the new line char when unmarshaling.
	return mt, string(m[1 : len(m)-1]), mt.Validate()
}

func (m *Message) MarshalString(mt MessageType, body string) error {
	buf := append([]byte{byte(mt)}, []byte(body)...)
	buf = append(buf, ByteLF)
	*m = Message(buf)
	return mt.Validate()
}

func (m *Message) MarshalMap(mt MessageType, data map[string]string) error {
	// Convert map to string format: key=value;key2=value2;...
	var parts []string
	for key, value := range data {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	body := strings.Join(parts, ";")

	return m.MarshalString(mt, body)
}

func (m Message) UnmarshalMap() (MessageType, map[string]string, error) {
	mt, body, err := m.UnmarshalString()
	if err != nil {
		return mt, nil, err
	}

	data := make(map[string]string)
	parts := strings.Split(body, ";")
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			data[kv[0]] = kv[1]
		}
	}

	return mt, data, nil
}

func newMessageString(mt MessageType, body string) (Message, error) {
	m := Message{}
	err := m.MarshalString(mt, body)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func newMessageMap(mt MessageType, body map[string]string) (Message, error) {
	m := Message{}
	err := m.MarshalMap(mt, body)
	if err != nil {
		return nil, err
	}

	return m, nil
}

func NewMsgRelayHello(regions map[string]string) Message {
	m, _ := newMessageMap(MsgRelayHello, regions)
	return m
}

func NewMsgRelayConfig(id string) Message {
	m, _ := newMessageString(MsgRelayConfig, id)
	return m
}

func NewMsgGatewayProxy(destination, region string) Message {
	m, _ := newMessageMap(MsgGatewayProxy, map[string]string{
		"destination": destination,
		"region":      region,
	})
	return m
}

func NewMsgSuccess() Message {
	m, _ := newMessageString(MsgSuccess, "")
	return m
}
func NewMsgError(err string) Message {
	m, _ := newMessageString(MsgError, err)
	return m
}
