package skill

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hjhsamuel/magent/pkg/fx"
)

func Install(url string, tempDir string, skillDir string) (*Frontmatter, error) {
	// 创建临时目录
	tempRoot, err := os.MkdirTemp(tempDir, "skill-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempRoot)

	// 下载压缩包
	archiveFile, err := fx.DownloadFileByUrl(url, tempRoot, tempRoot)
	if err != nil {
		return nil, err
	}

	// 解压
	tempSkillDir := filepath.Join(tempDir, "skill")
	_ = os.MkdirAll(tempSkillDir, os.ModePerm)
	err = fx.ExtractDownloadedArchive(archiveFile, tempSkillDir)
	if err != nil {
		return nil, err
	}

	// 解析并校验 skill
	var skillFile string
	err = filepath.WalkDir(tempSkillDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "SKILL.md" {
			skillFile = path
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if skillFile == "" {
		return nil, errors.New("no SKILL.md found")
	}
	// 读取 frontmatter
	fm, _, err := ReadSkill(skillFile, false)
	if err != nil {
		return nil, err
	}

	// 将 skill 移动至指定目录
	err = os.Rename(filepath.Dir(skillFile), filepath.Join(skillDir, fm.Name))
	if err != nil {
		return nil, err
	}

	return fm, nil
}
