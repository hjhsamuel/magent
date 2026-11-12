package service

import "github.com/hjhsamuel/magent/api/utils"

func (s *Service) Chat(
	auth *utils.Auth,
	sessionId string,
	content string,
) error {
	// TODO
	panic("implement me")
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
