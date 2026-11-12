package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/runtime"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) Chat(
	auth *utils.Auth,
	sessionId string,
	content string,
) error {
	sid, err := bson.ObjectIDFromHex(sessionId)
	if err != nil {
		return err
	}
	obj, err := s.store.GetSession(
		context.Background(),
		bson.M{
			"_id":  sid,
			"user": fmt.Sprintf("%v", auth.Raw["id"]),
		},
	)
	if err != nil {
		return err
	}

	if obj.Status == schema.SessionStateRunning {
		return errors.New("already running")
	}

	params := runtime.StartTurnParams{
		SessionID: obj.RunId,
		Input: []runtime.Input{
			{Type: "text", Text: content},
		},
	}
	if mr, exists := s.manager.Get(sessionId); exists {
		_, err = mr.StartTurn(context.Background(), params)
		if err != nil {
			return err
		}
	} else {
		r, err := s.initRuntime(context.Background(), obj.Node)
		if err != nil {
			return err
		}
		defer r.Close()

		_, err = r.StartTurn(context.Background(), params)
		if err != nil {
			return err
		}
	}

	_ = s.store.UpdateSession(
		context.Background(),
		bson.M{"_id": sid},
		bson.M{"$set": bson.M{"status": schema.SessionStateRunning}},
	)

	return nil
}

func (s *Service) Approve(
	auth *utils.Auth,
	sessionId string,
	id string,
	content string,
) error {

	// TODO
	panic("implement me")
}
