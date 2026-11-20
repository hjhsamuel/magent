package service

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) AddMcp(auth *utils.Auth, obj *schema.Tool) (string, error) {
	obj.Kind = schema.ToolMcp
	obj.Status = schema.ToolStateOffline
	obj.Creator = auth.ID
	obj.CreatedAt = time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, err := s.store.AddTool(ctx, obj)
	if err != nil {
		return "", err
	}
	return id.Hex(), nil
}

func (s *Service) SetMcpStatus(id string, status schema.ToolState) error {
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

	// TODO

	return id.Hex(), nil
}

func (s *Service) RemoveSkill(id string) error {
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

	_ = os.RemoveAll(filepath.Join("workspace", "skills", obj.Name))
	return nil
}
