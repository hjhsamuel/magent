package service

import (
	"context"
	"time"

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
		"user":   auth.ID,
		"status": bson.M{"$ne": schema.SessionStateDeleted},
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

func (s *Service) DeleteSession(auth *utils.Auth, id string) error {
	sid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	obj, err := s.store.FindAndUpdateOneSession(
		context.Background(),
		bson.M{"_id": sid, "user": auth.ID, "status": schema.SessionStateCompleted},
		bson.M{"$set": bson.M{"status": schema.SessionStateDeleted}},
	)
	if err != nil {
		return err
	}

	ur, err := s.cm.CreateOrGet(auth.ID, s.getWebsocketHeaders())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 不关心是否真的删除成功
	_, _ = ur.DeleteThreads(ctx, obj.RunId)
	return nil
}
