package schema

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const TurnCollection = "magent_turn"

type Turn struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Session   bson.ObjectID `bson:"session"`
	CreatedAt time.Time     `bson:"created_at"`
	Messages  []*Message    `bson:"messages"`
}

type Message struct {
	Role      MessageRole `bson:"role"`
	Content   string      `bson:"content"`
	CreatedAt time.Time   `bson:"created_at"`
}

type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"       // 用户普通输入
	MessageRoleModel     MessageRole = "model"      // 模型输出
	MessageRoleUserInput MessageRole = "user-input" // 用户审批
)
