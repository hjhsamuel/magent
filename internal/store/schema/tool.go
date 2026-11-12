package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const McpCollection = "magent_mcp"

type Mcp struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Url       string        `bson:"url"`
	Status    ToolState     `bson:"status"`
	Token     string        `bson:"token"` // bearer token
	Creator   string        `bson:"creator"`
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

type ToolState int

const (
	ToolStateOnline ToolState = iota
	ToolStateOffline
)
