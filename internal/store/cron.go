package store

import (
	"context"

	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Store) GetCronJobs(
	ctx context.Context,
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*schema.Cron, error) {
	cursor, err := d.getCollection(schema.CronCollection).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}

	var objs []*schema.Cron
	if err = cursor.All(ctx, &objs); err != nil {
		return nil, err
	}

	return objs, nil
}

func (d *Store) AddCronJob(ctx context.Context, obj *schema.Cron) (bson.ObjectID, error) {
	result, err := d.getCollection(schema.CronCollection).InsertOne(ctx, obj)
	if err != nil {
		return bson.ObjectID{}, err
	}

	id := result.InsertedID.(bson.ObjectID)
	return id, nil
}

func (d *Store) DeleteCronJob(ctx context.Context, filter bson.M) error {
	result, err := d.getCollection(schema.CronCollection).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (d *Store) UpdateCronJob(
	ctx context.Context,
	filter bson.M,
	update bson.M,
	opts ...options.Lister[options.FindOneAndUpdateOptions],
) (*schema.Cron, error) {
	result := d.getCollection(schema.CronCollection).FindOneAndUpdate(ctx, filter, update, opts...)
	if err := result.Err(); err != nil {
		return nil, err
	}

	var obj schema.Cron
	if err := result.Decode(&obj); err != nil {
		return nil, err
	}
	return &obj, nil
}
