package codex

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type Server struct {
	pid     int
	user    string
	workdir string

	wg sync.WaitGroup
}

func (s *Server) Start() error {
	cmd := exec.Command(
		"codex",
		"app-server",
		"--listen",
		fmt.Sprintf("unix:///run/codex/%s.sock", s.user),
	)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("CODEX_HOME=%s/.codex", s.workdir),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	s.pid = cmd.Process.Pid

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		_ = cmd.Wait()
	}()

	return nil
}

func (s *Server) Close() {
	_ = syscall.Kill(-s.pid, syscall.SIGTERM)
	s.wg.Wait()
}

func NewCodexServer(user string, rootDir string) error {
	// TODO
}
