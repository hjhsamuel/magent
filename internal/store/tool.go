package store

import (
	"context"

	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Store) FindTools(
	ctx context.Context,
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*schema.Tool, error) {
	cursor, err := d.getCollection(schema.ToolCollection).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var objs []*schema.Tool
	if err = cursor.All(ctx, &objs); err != nil {
		return nil, err
	}
	return objs, nil
}

func (d *Store) AddTool(ctx context.Context, obj *schema.Tool) (bson.ObjectID, error) {
	result, err := d.getCollection(schema.ToolCollection).InsertOne(ctx, obj)
	if err != nil {
		return bson.ObjectID{}, err
	}

	id := result.InsertedID.(bson.ObjectID)
	return id, nil
}

func (d *Store) UpdateTool(ctx context.Context, filter, update bson.M) error {
	result, err := d.getCollection(schema.ToolCollection).UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.ModifiedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (d *Store) RemoveTool(
	ctx context.Context,
	filter bson.M,
	opts ...options.Lister[options.FindOneAndDeleteOptions],
) (*schema.Tool, error) {
	result := d.getCollection(schema.ToolCollection).FindOneAndDelete(ctx, filter, opts...)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.Tool
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}
	return &obj, nil
}
