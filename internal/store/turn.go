package store

import (
	"context"
	"time"

	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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

	curTime := time.Now()
	_, err = session.WithTransaction(ctx, func(sc context.Context) (any, error) {
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

func (d *Store) StartTurn(
	ctx context.Context,
	sessionId bson.ObjectID,
	content string,
) (bson.ObjectID, error) {
	session, err := d.StartSession()
	if err != nil {
		return bson.ObjectID{}, err
	}
	defer session.EndSession(ctx)

	var id bson.ObjectID
	now := time.Now()
	_, err = session.WithTransaction(ctx, func(sc context.Context) (any, error) {
		result, err := d.getCollection(schema.TurnCollection).InsertOne(sc, &schema.Turn{
			Session:   sessionId,
			CreatedAt: now,
			Messages: []*schema.Message{
				{
					Role:      schema.MessageRoleUser,
					Content:   content,
					CreatedAt: now,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		id = result.InsertedID.(bson.ObjectID)

		_, err = d.getCollection(schema.SessionCollection).UpdateOne(
			sc,
			bson.M{"_id": sessionId, "status": bson.M{"$ne": schema.SessionStateRunning}},
			bson.M{"$set": bson.M{"status": schema.SessionStateRunning, "updated_at": now}},
		)
		if err != nil {
			return nil, err
		}

		return nil, nil
	})
	if err != nil {
		return bson.ObjectID{}, err
	}

	return id, nil
}

func (d *Store) UpdateTurn(ctx context.Context, filter, update bson.M) error {
	_, err := d.getCollection(schema.TurnCollection).UpdateOne(ctx, filter, update)
	return err
}

func (d *Store) GetTurnList(
	ctx context.Context,
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*schema.Turn, error) {
	cursor, err := d.getCollection(schema.TurnCollection).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var objs []*schema.Turn
	if err = cursor.All(ctx, &objs); err != nil {
		return nil, err
	}
	return objs, nil

}
