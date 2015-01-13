// Copyright (2014) Sandia Corporation.
// Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation,
// the U.S. Government retains certain rights in this software.

package ron

import (
	"encoding/gob"
	"fmt"
	"goserial"
	log "minilog"
)

const (
	BAUDRATE = 115200
)

func (r *Ron) serialDial() error {
	c := &serial.Config{
		Name: r.serialPath,
		Baud: BAUDRATE,
	}

	s, err := serial.OpenPort(c)
	if err != nil {
		return err
	}

	r.serialClientHandle = s

	return nil
}

func (r *Ron) serialHeartbeat(h *hb) (map[int]*Command, error, bool) {
	if r.serialClientHandle == nil {
		log.Fatalln("no serial handle!")
	}

	enc := gob.NewEncoder(r.serialClientHandle)

	err := enc.Encode(h)
	if err != nil {
		return nil, err, false
	}

	newCommands := make(map[int]*Command)
	dec := gob.NewDecoder(r.serialClientHandle)

	err = dec.Decode(&newCommands)
	if err != nil {
		return nil, err, true
	}

	return newCommands, nil, true
}

// Dial a client serial port. Used by a master ron node only.
func (r *Ron) SerialDialClient(path string) error {
	log.Debug("SerialDialClient: %v", path)

	if r.mode != MODE_MASTER {
		log.Fatalln("SerialDialClient must be in master mode")
	}

	r.serialLock.Lock()
	defer r.serialLock.Unlock()

	// are we already connected to this client?
	if _, ok := r.masterSerialConns[path]; ok {
		return fmt.Errorf("already connected to serial client %v", path)
	}

	// connect!
	c := &serial.Config{
		Name: r.serialPath,
		Baud: BAUDRATE,
	}

	s, err := serial.OpenPort(c)
	if err != nil {
		return err
	}

	r.masterSerialConns[path] = s

	go r.serialClientHandler(path)

	return nil
}

func (r *Ron) serialClientHandler(path string) {
	log.Debug("serialClientHandler: %v", path)

	r.serialLock.Lock()
	c, ok := r.masterSerialConns[path]
	r.serialLock.Unlock()

	if !ok {
		log.Fatal("could not access client: %v", path)
	}

	dec := gob.NewDecoder(c)

	for {
		var h hb
		err := dec.Decode(&h)
		if err != nil {
			log.Errorln(err)
			break
		}
		log.Debug("heartbeat from %v", h.UUID)

		// process the heartbeat in a goroutine so we can send the command list back faster
		go r.masterHeartbeat(&h)

		// send the command list back
		buf, err := r.encodeCommands()
		if err != nil {
			log.Errorln(err)
			break
		}
		c.Write(buf)
	}

	// remove this path from the list of connected serial ports
	log.Debug("disconnecting serial client: %v", path)

	r.serialLock.Lock()
	delete(r.masterSerialConns, path)
	r.serialLock.Unlock()
}
