package store

import (
	"context"

	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (d *Store) FindMCPs(
	ctx context.Context,
	filter bson.M,
	opts ...options.Lister[options.FindOptions],
) ([]*schema.Mcp, error) {
	cursor, err := d.getCollection(schema.McpCollection).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var objs []*schema.Mcp
	if err = cursor.All(ctx, &objs); err != nil {
		return nil, err
	}
	return objs, nil
}
