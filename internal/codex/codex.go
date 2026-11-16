package codex

import (
	"context"
	"encoding/json"
)

type Runtime struct {
	user, thread string
	conn         MessageTransport
}

func (r *Runtime) init() error {
	// initialize
}

func NewRuntime(sockPath, user, thread string) (*Runtime, error) {
	c, err := NewUnixConn(sockPath)
	if err != nil {
		return nil, err
	}

	r := &Runtime{
		user:   user,
		thread: thread,
		conn:   c,
	}
	if err = r.init(); err != nil {
		_ = r.conn.Close()
		return nil, err
	}

	return r, nil
}
