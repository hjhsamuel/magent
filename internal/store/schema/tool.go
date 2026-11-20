package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const ToolCollection = "magent_tool"

type Tool struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Name        string        `bson:"name"`
	Description string        `bson:"description"`
	Url         string        `bson:"url"`
	Kind        ToolKind      `bson:"kind"`
	Status      ToolState     `bson:"status"`
	Token       string        `bson:"token"` // bearer token
	Creator     int           `bson:"creator"`
	CreatedAt   time.Time     `bson:"created_at"`
	UpdatedAt   time.Time     `bson:"updated_at"`
}

type ToolKind int

const (
	ToolMcp ToolKind = iota
	ToolSkill
)

type ToolState int

const (
	ToolStateOnline ToolState = iota
	ToolStatusLoading
	ToolStateOffline
)
