package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const SessionCollection = "magent_session"

type Session struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	User      string        `bson:"user"`
	Status    SessionState  `bson:"status"`
	Node      string        `bson:"node"`   // node server
	RunId     string        `bson:"run_id"` // 映射到 runtime 的真实 id
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

type SessionState int

const (
	SessionStateInit SessionState = iota
	SessionStateRunning
	SessionStateCompleted
)
