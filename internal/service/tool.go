package service

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/mcpx"
	"github.com/hjhsamuel/magent/internal/skill"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) AddMcp(auth *utils.Auth, obj *schema.Tool) (string, error) {
	obj.Kind = schema.ToolMcp
	obj.Creator = auth.ID
	obj.CreatedAt = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mcpSrv, err := mcpx.NewMCP(obj.Url, nil)
	if err != nil {
		return "", err
	}
	if err = mcpSrv.Ping(ctx); err != nil {
		return "", err
	}

	obj.Status = schema.ToolStatusReady

	id, err := s.store.AddTool(context.Background(), obj)
	if err != nil {
		return "", err
	}
	return id.Hex(), nil
}

func (s *Service) SetMcpStatus(id string, status schema.ToolState) error {
	// TODO
	tid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.store.UpdateTool(
		ctx,
		bson.M{"_id": tid, "kind": schema.ToolMcp},
		bson.M{"$set": bson.M{"status": status, "updated_at": time.Now()}},
	)
}

func (s *Service) RemoveMcp(id string) error {
	// TODO
	tid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = s.store.RemoveTool(ctx, bson.M{"_id": tid, "kind": schema.ToolMcp})
	return err
}

func (s *Service) AddSkill(auth *utils.Auth, obj *schema.Tool) (string, error) {
	obj.Kind = schema.ToolSkill
	obj.Status = schema.ToolStatusLoading
	obj.Creator = auth.ID
	obj.CreatedAt = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, err := s.store.AddTool(ctx, obj)
	if err != nil {
		return "", err
	}

	go s.downloadAndInstallSkill(id, obj.Url)

	return id.Hex(), nil
}

func (s *Service) downloadAndInstallSkill(id bson.ObjectID, url string) {
	var (
		filter = bson.M{"_id": id}
		update bson.M
	)
	fm, err := skill.Install(
		url,
		filepath.Join(WorkspaceDir, TempDirName),
		filepath.Join(WorkspaceDir, SkillDirName),
	)
	if err != nil {
		update = bson.M{"$set": bson.M{
			"status":     schema.ToolStateOffline,
			"error":      err.Error(),
			"updated_at": time.Now(),
		}}
	} else {
		update = bson.M{"$set": bson.M{
			"status":      schema.ToolStatusReady,
			"name":        fm.Name,
			"description": fm.Description,
			"updated_at":  time.Now(),
		}}
	}
	_ = s.store.UpdateTool(context.Background(), filter, update)
}

func (s *Service) RemoveSkill(id string) error {
	// TODO
	tid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	obj, err := s.store.RemoveTool(ctx, bson.M{"_id": tid, "kind": schema.ToolSkill})
	if err != nil {
		return err
	}

	if obj.Name != "" && obj.Status == schema.ToolStateOnline {
		_ = os.RemoveAll(filepath.Join(WorkspaceDir, SkillDirName, obj.Name))
	}

	return nil
}
