package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hjhsamuel/magent/api/utils"
	"github.com/hjhsamuel/magent/internal/codex"
	"github.com/hjhsamuel/magent/internal/store/schema"
	"github.com/hjhsamuel/magent/pkg/cryptox"
	"github.com/hjhsamuel/magent/pkg/fx"
	"github.com/hjhsamuel/magent/pkg/token"
	"github.com/pelletier/go-toml/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func (s *Service) initUserWorkspace(parent context.Context, auth *utils.Auth) (*codex.UserRuntime, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()

	// 检测 pod 是否存在
	podName := fmt.Sprintf("codex-%d", auth.ID)
	namespace := s.k8s.Namespace()
	_, err := s.k8s.GetPod(ctx, namespace, podName)
	missing := apierrors.IsNotFound(err)
	if err != nil && !missing {
		return nil, err
	}

	if missing {
		userToken, err := token.GenerateJWTWithoutExp(auth.Raw, utils.GetJwtSalt())
		if err != nil {
			return nil, err
		}

		// 创建用户工作目录
		if err = s.createUserWorkspace(auth); err != nil {
			return nil, err
		}

		// 创建pod
		automountToken, hostUsers := false, false
		procMount := corev1.UnmaskedProcMount
		_, err = s.k8s.CreatePod(
			ctx,
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: namespace,
					Labels: map[string]string{
						s.runtimePod.Label.Key: s.runtimePod.Label.Value,
					},
				},
				Spec: corev1.PodSpec{
					Hostname:                     podName,
					Subdomain:                    s.runtimePod.SubDomain,
					AutomountServiceAccountToken: &automountToken,
					// An unmasked proc mount requires a private user namespace.
					HostUsers: &hostUsers,
					NodeSelector: map[string]string{
						s.runtimePod.Label.Key: s.runtimePod.Label.Value,
					},
					Containers: []corev1.Container{{
						Name:  podName,
						Image: s.runtimePod.Image,
						SecurityContext: &corev1.SecurityContext{
							AppArmorProfile: &corev1.AppArmorProfile{Type: corev1.AppArmorProfileTypeUnconfined},
							SeccompProfile:  &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined},
							ProcMount:       &procMount,
						},
						Command: []string{
							"codex",
							"app-server",
							"--listen",
							"ws://0.0.0.0:8000",
							"--ws-auth",
							"capability-token",
							"--ws-token-sha256",
							"$(TOKEN_SHA256)",
						},
						Env: []corev1.EnvVar{
							{Name: "CODEX_HOME", Value: "/app/.codex"},
							{Name: "TOKEN_SHA256", Value: cryptox.TokenSha256(s.wsToken)},
							{Name: "MAGENT_USER_TOKEN", Value: userToken},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "workspace",
								MountPath: "/app/.codex",
								SubPath:   fmt.Sprintf("%s/%d", UserWorkDirName, auth.ID),
							},
							{
								Name:      "workspace",
								MountPath: "/root/.agents/skills",
								SubPath:   SkillDirName,
								ReadOnly:  true,
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: s.runtimePod.Pvc,
								},
							},
						},
					},
				},
			},
		)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return nil, err
		}
	}
	err = wait.PollUntilContextTimeout(
		parent,
		time.Second*3,
		time.Minute,
		true,
		func(ctx context.Context) (bool, error) {
			pod, err := s.k8s.GetPod(ctx, namespace, podName)
			if err != nil {
				return false, err
			}
			switch pod.Status.Phase {
			case corev1.PodRunning:
				return true, nil
			case corev1.PodFailed, corev1.PodSucceeded:
				_ = s.k8s.DeletePod(ctx, pod)
				return false, fmt.Errorf("pod %s, reason: %s, message: %s", pod.Status.Phase, pod.Status.Reason,
					pod.Status.Message)
			default:
				return false, nil
			}
		},
	)
	if err != nil {
		return nil, fmt.Errorf("starting workspace pod %s/%s: %w", namespace, podName, err)
	}

	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+s.wsToken)
	return s.cm.CreateOrGet(auth.ID, headers)
}

func (s *Service) createUserWorkspace(auth *utils.Auth) error {
	path, exists := s.isUserWorkspaceExists(auth)
	if exists {
		return nil
	}

	_ = os.MkdirAll(path, os.ModePerm)

	if err := fx.CopyFile(
		filepath.Join(WorkspaceDir, "models.json"),
		filepath.Join(path, "models.json"),
	); err != nil {
		return err
	}

	if err := fx.CopyFile(
		filepath.Join(WorkspaceDir, "config.toml"),
		filepath.Join(path, "config.toml"),
	); err != nil {
		return err
	}

	return nil
}

func (s *Service) updateAllUserWorkspace(hasMcp, hasSkill bool) error {
	// 更新文件
	var (
		rootPath = filepath.Join(WorkspaceDir, UserWorkDirName)
		cfgPath  = filepath.Join(WorkspaceDir, "config.toml")
	)
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return err
	}
	for _, item := range entries {
		_ = fx.CopyFile(
			cfgPath,
			filepath.Join(rootPath, item.Name()),
		)
	}
	// 获取 pod
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pods, err := s.k8s.GetPodList(ctx, s.k8s.Namespace(), s.runtimePod.Label.Key+"="+s.runtimePod.Label.Value)
	if err != nil {
		return err
	}
	// 发送信号
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer "+s.wsToken)
	for _, item := range pods {
		strId := strings.TrimPrefix(item.Name, "codex-")
		userId, err := strconv.Atoi(strId)
		if err != nil {
			continue
		}
		ur, err := s.cm.CreateOrGet(userId, headers)
		if err != nil {
			continue
		}
		_ = ur.ReloadSkillsAndMCPServers(context.Background(), hasMcp, hasSkill)
	}

	return nil
}

func (s *Service) updateWorkspaceConfig(tools ...*schema.Tool) (bool, bool, error) {
	if len(tools) == 0 {
		return false, false, nil
	}

	var (
		addMcps, delMcps       = make([]*schema.Tool, 0), make([]*schema.Tool, 0)
		updateMcp, updateSkill = false, false
	)
	for _, item := range tools {
		if item.Status != schema.ToolStatusReady && item.Status != schema.ToolStateUninstall {
			continue
		}

		switch item.Kind {
		case schema.ToolMcp:
			updateMcp = true
			// mcp 需要更新用户配置
			if item.Status == schema.ToolStatusReady {
				addMcps = append(addMcps, item)
			} else if item.Status == schema.ToolStateUninstall {
				delMcps = append(delMcps, item)
			}
		case schema.ToolSkill:
			// skill 目录已经更新，只需要发送信号即可
			updateSkill = true
		}
	}

	// 更新 config.toml
	if updateMcp {
		cfg, err := s.getWorkspaceConfig(addMcps, delMcps)
		if err != nil {
			return false, false, err
		}
		content, err := toml.Marshal(cfg)
		if err != nil {
			return false, false, err
		}
		err = os.WriteFile(filepath.Join(WorkspaceDir, "config.toml"), content, 0644)
		if err != nil {
			return false, false, err
		}
	}

	return updateMcp, updateSkill, nil
}

func (s *Service) isUserWorkspaceExists(auth *utils.Auth) (string, bool) {
	path := filepath.Join(WorkspaceDir, UserWorkDirName, strconv.Itoa(auth.ID))
	if _, err := os.Stat(path); err != nil {
		return path, false
	}
	if _, err := os.Stat(filepath.Join(path, "config.toml")); err != nil {
		return path, false
	}
	return path, true
}

func (s *Service) getWorkspaceConfig(addMcps, delMcps []*schema.Tool) (*codex.Config, error) {
	path := filepath.Join(WorkspaceDir, "config.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg *codex.Config
	if err = toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}

	for _, item := range delMcps {
		delete(cfg.McpServers, item.Name)
	}
	for _, item := range addMcps {
		info := codex.McpServer{
			Url:         item.Url,
			HttpHeaders: make(map[string]string),
		}
		if item.Token != "" {
			info.HttpHeaders["Authorization"] = "Bearer " + item.Token
		} else {
			info.BearerTokenEnvVar = "MAGENT_USER_TOKEN"
		}
		cfg.McpServers[item.Name] = info
	}
	return cfg, nil
}

func (s *Service) codexAddress(id int) string {
	return fmt.Sprintf("codex-%d.%s:8000", id, s.runtimePod.SubDomain)
}

func (s *Service) getEndpoint(id int) (string, error) {
	return "ws://" + s.codexAddress(id), nil
}
