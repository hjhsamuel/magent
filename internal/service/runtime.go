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

	if obj.Status != schema.SessionStateCompleted {
		return errors.New("already running")
	}

	mr, exists := s.manager.Get(sessionId)
	if !exists {
		return errors.New("sse not connected")
	}
	_, err = mr.StartTurn(context.Background(), runtime.StartTurnParams{
		SessionID: obj.RunId,
		Input: []runtime.Input{
			{Type: "text", Text: content},
		},
	})
	if err != nil {
		return err
	}

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
