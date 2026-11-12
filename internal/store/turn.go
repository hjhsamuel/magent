package store

import (
	"context"
	"time"

	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (d *Store) TurnComplete(
	ctx context.Context,
	runId string,
	turnId string,
	content string,
) error {
	session, err := d.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		curTime := time.Now()
		_, err = d.getCollection(schema.TurnCollection).UpdateOne(
			sc,
			bson.M{"turn_id": turnId},
			bson.M{"$push": bson.M{"messages": &schema.Message{
				Role:      schema.MessageRoleModel,
				Content:   content,
				CreatedAt: curTime,
			}}},
		)
		if err != nil {
			return nil, err
		}

		_, err = d.getCollection(schema.SessionCollection).UpdateOne(
			sc,
			bson.M{"run_id": runId},
			bson.M{"$set": bson.M{
				"status":     schema.SessionStateCompleted,
				"updated_at": curTime,
			}},
		)
		if err != nil {
			return nil, err
		}

		return nil, nil
	})
	return err

}
