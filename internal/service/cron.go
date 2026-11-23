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

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.exit:
			return
		case t := <-ticker.C:
			tools, err := s.store.FindTools(
				context.Background(),
				bson.M{"status": bson.M{"$in": []schema.ToolState{
					schema.ToolStatusReady,
					schema.ToolStateUninstall,
				}}},
			)
			if err != nil {
				logrus.WithError(err).Error("get tools")
				continue
			}

			updateMcp, updateSkill, err := s.updateWorkspaceConfig(tools...)
			if err != nil {
				logrus.WithError(err).Error("update workspace")
				continue
			}

			if err = s.updateAllUserWorkspace(updateMcp, updateSkill); err != nil {
				logrus.WithError(err).Error("update user workspace")
			}

			var (
				onlineIds  = make([]bson.ObjectID, 0)
				offlineIds = make([]bson.ObjectID, 0)
			)
			for _, item := range tools {
				switch item.Status {
				case schema.ToolStatusReady:
					onlineIds = append(onlineIds, item.ID)
				case schema.ToolStateUninstall:
					offlineIds = append(offlineIds, item.ID)
				}
			}
			if len(onlineIds) != 0 {
				_, _ = s.store.UpdateTool(
					context.Background(),
					bson.M{"_id": bson.M{"$in": onlineIds}},
					bson.M{"$set": bson.M{
						"status":     schema.ToolStateOnline,
						"updated_at": t,
					}},
				)
			}
			if len(offlineIds) != 0 {
				_, _ = s.store.RemoveTool(
					context.Background(),
					bson.M{"_id": bson.M{"$in": offlineIds}},
				)
			}
		}
	}
}
