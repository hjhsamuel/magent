package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const SessionCollection = "magent_session"

type Session struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Status    SessionState  `bson:"state"`
	CreatedAt time.Time     `bson:"created_at"`
}

type SessionState int

const (
	SessionStateRunning SessionState = iota
	SessionStateCompleted
)
