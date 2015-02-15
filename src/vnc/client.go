package vnc

import (
	"encoding/binary"
	"fmt"
	"io"
)

// See RFC 6143 Section 7.5
const (
	TypeSetPixelFormat uint8 = iota
	_                        // Not used
	TypeSetEncodings
	TypeFramebufferUpdateRequest
	TypeKeyEvent
	TypePointerEvent
	TypeClientCutText
)

var clientMessages = map[uint8]func() interface{}{
	TypeSetPixelFormat:           func() interface{} { return new(SetPixelFormat) },
	TypeSetEncodings:             func() interface{} { return new(_SetEncodings) },
	TypeFramebufferUpdateRequest: func() interface{} { return new(FramebufferUpdateRequest) },
	TypeKeyEvent:                 func() interface{} { return new(KeyEvent) },
	TypePointerEvent:             func() interface{} { return new(PointerEvent) },
	TypeClientCutText:            func() interface{} { return new(_ClientCutText) },
}

// See RFC 6143 Section 7.4
type PixelFormat struct {
	BitsPerPixel, Depth, BigEndianFlag, TrueColorFlag uint8
	RedMax, GreenMax, BlueMax                         uint16
	RedShift, GreenShift, BlueShift                   uint8
	_                                                 [3]byte // Padding
}

// See RFC 6143 Section 7.5.1
type SetPixelFormat struct {
	_ [3]byte // Padding
	PixelFormat
}

type _SetEncodings struct {
	_                 [1]byte // Padding
	NumberOfEncodings uint16  // Length of Encodings
}

// See RFC 6143 Section 7.5.2
type SetEncodings struct {
	_SetEncodings
	Encodings []int32
}

// See RFC 6143 Section 7.5.3
type FramebufferUpdateRequest struct {
	Incremental uint8
	XPosition   uint16
	YPosition   uint16
	Width       uint16
	Height      uint16
}

// See RFC 6143 Section 7.5.4
type KeyEvent struct {
	DownFlag uint8
	_        [2]byte // Padding
	Key      uint32
}

// See RFC 6143 Section 7.5.5
type PointerEvent struct {
	ButtonMask uint8
	XPosition  uint16
	YPosition  uint16
}

type _ClientCutText struct {
	_      [3]byte // Padding
	Length uint32  // Length of Text
}

// See RFC 6143 Section 7.5.6
type ClientCutText struct {
	_ClientCutText
	Text []uint8
}

func (m *SetPixelFormat) Write(w io.Writer) error {
	return writeMessage(w, TypeSetPixelFormat, m)
}

func (m *SetEncodings) Write(w io.Writer) error {
	// Ensure length is set correctly
	m.NumberOfEncodings = uint16(len(m.Encodings))

	if err := writeMessage(w, TypeSetEncodings, m._SetEncodings); err != nil {
		return err
	}

	// Write variable length encodings
	if err := binary.Write(w, binary.BigEndian, &m.Encodings); err != nil {
		return fmt.Errorf("unable to write encodings -- %s", err.Error())
	}

	return nil
}

func (m *FramebufferUpdateRequest) Write(w io.Writer) error {
	return writeMessage(w, TypeFramebufferUpdateRequest, m)
}

func (m *KeyEvent) Write(w io.Writer) error {
	return writeMessage(w, TypeKeyEvent, m)
}

func (m *PointerEvent) Write(w io.Writer) error {
	return writeMessage(w, TypePointerEvent, m)
}

func (m *ClientCutText) Write(w io.Writer) error {
	// Ensure length is set correctly
	m.Length = uint32(len(m.Text))

	if err := writeMessage(w, TypeClientCutText, m._ClientCutText); err != nil {
		return err
	}

	// Write variable length encodings
	if err := binary.Write(w, binary.BigEndian, &m.Text); err != nil {
		return fmt.Errorf("unable to write text -- %s", err.Error())
	}

	return nil
}

// ReadClientMessage reads the next client-to-server message
func ReadClientMessage(r io.Reader) (interface{}, error) {
	var msgType uint8
	if err := binary.Read(r, binary.BigEndian, &msgType); err != nil {
		return nil, fmt.Errorf("unable to read message type -- %s", err.Error())
	}

	if _, ok := clientMessages[msgType]; !ok {
		return nil, fmt.Errorf("unknown client-to-server message: %d", msgType)
	}

	// Copy the struct
	msg := clientMessages[msgType]()

	if err := binary.Read(r, binary.BigEndian, msg); err != nil {
		return nil, fmt.Errorf("unable to read message -- %s", err.Error())
	}

	// Do extra processing on messages that have variable length fields
	switch msgType {
	case TypeSetEncodings:
		newMsg := &SetEncodings{_SetEncodings: *msg.(*_SetEncodings)}
		newMsg.Encodings = make([]int32, newMsg.NumberOfEncodings)

		if err := binary.Read(r, binary.BigEndian, &newMsg.Encodings); err != nil {
			return nil, fmt.Errorf("unable to read encodings -- %s", err.Error())
		}

		msg = newMsg
	case TypeClientCutText:
		newMsg := &ClientCutText{_ClientCutText: *msg.(*_ClientCutText)}
		newMsg.Text = make([]uint8, newMsg.Length)

		if err := binary.Read(r, binary.BigEndian, &newMsg.Text); err != nil {
			return nil, fmt.Errorf("unable to read text -- %s", err.Error())
		}

		msg = newMsg
	}

	return msg, nil
}
