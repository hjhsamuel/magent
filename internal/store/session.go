package store

import (
	"context"

	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Store) GetSession(
	ctx context.Context,
	filter bson.M,
	opts ...options.Lister[options.FindOneOptions],
) (*schema.Session, error) {
	result := d.getCollection(schema.SessionCollection).FindOne(ctx, filter, opts...)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.Session
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}
	return &obj, nil
}

func (d *Store) CreateSession(
	ctx context.Context,
	obj *schema.Session,
) (bson.ObjectID, error) {
	result, err := d.getCollection(schema.SessionCollection).InsertOne(ctx, obj)
	if err != nil {
		return bson.ObjectID{}, err
	}

	id := result.InsertedID.(bson.ObjectID)
	return id, nil
}

func (d *Store) UpdateSession(
	ctx context.Context,
	filter bson.M,
	update bson.M,
) error {
	_, err := d.getCollection(schema.SessionCollection).UpdateOne(ctx, filter, update)
	return err
}
