package service

import (
	"context"

	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func (s *Service) GetSessionList(
	auth *utils.Auth,
	offset string,
	limit int64,
) ([]*schema.Session, error) {
	filter := bson.M{
		"user": auth.ID,
	}
	if offset != "" {
		id, err := bson.ObjectIDFromHex(offset)
		if err != nil {
			return nil, err
		}
		filter["_id"] = bson.M{"$lt": id}
	}

	return s.store.FindSession(
		context.Background(),
		filter,
		options.Find().SetSort(bson.M{"_id": -1}).SetLimit(limit),
	)
}

func (s *Service) GetTurns(
	auth *utils.Auth,
	sessionId string,
	offset string,
	limit int64,
) ([]*schema.Turn, error) {
	sid, err := bson.ObjectIDFromHex(sessionId)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"session": sid}

	if offset != "" {
		turnId, err := bson.ObjectIDFromHex(offset)
		if err != nil {
			return nil, err
		}
		filter["_id"] = bson.M{"$lt": turnId}
	}

	return s.store.GetTurnList(
		context.Background(),
		filter,
		options.Find().SetSort(bson.M{"_id": -1}).SetLimit(limit),
	)
}
