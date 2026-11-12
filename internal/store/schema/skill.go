package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const SkillCollection = "magent_skill"

type Skill struct {
	ID          bson.ObjectID `bson:"_id,omitempty"`
	Name        string        `bson:"name"`
	Description string        `bson:"description"`
	Url         string        `bson:"url"`
	Creator     string        `bson:"creator"`
	CreatedAt   time.Time     `bson:"created_at"`
}
