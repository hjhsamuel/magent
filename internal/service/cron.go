package service

import (
	"context"
	"time"

	"github.com/hjhsamuel/magent/internal/store/schema"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) initCron() error {
	s.wg.Add(1)
	go s.cronOnlineTools()

	return nil
}

func (s *Service) cronOnlineTools() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	for {
		select {
		case <-s.exit:
			return
		case t := <-ticker.C:
			ids, err := s.updateWorkspaceConfig()
			if err != nil {
				logrus.WithError(err).Error("update config.toml")
				continue
			}
			// TODO

			_ = s.store.UpdateTool(
				context.Background(),
				bson.M{"_id": bson.M{"$in": ids}},
				bson.M{"$set": bson.M{
					"status":     schema.ToolStateOnline,
					"updated_at": t,
				}},
			)
		}
	}
}
