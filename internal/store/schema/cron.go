package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const CronCollection = "magent_cron"

type Cron struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Schedule  string        `bson:"schedule"`
	Content   string        `bson:"content"`
	Status    CronState     `bson:"status"`
	Creator   int           `bson:"creator"`
	CreatedAt time.Time     `bson:"created_at"`
}

type CronState int

const (
	CronStateOnline CronState = iota
	CronStateOffline
)
