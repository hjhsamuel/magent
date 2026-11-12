package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var gConf = &Config{
	Server: &ServerConfig{
		Host: "0.0.0.0",
		Port: 8080,
	},
	Log: &LogConfig{
		Path:   "logs/magent.log",
		Level:  "info",
		Size:   50,
		Backup: 3,
	},
	Store: &StoreConfig{
		MaxIdleConn:     200,
		MinIdleConn:     5,
		MaxConnIdleTime: time.Minute * 5,
		Debug:           false,
	},
}

func GetConfig() *Config {
	return gConf
}

func Init() error {
	_ = godotenv.Load()

	if err := setServer(); err != nil {
		return err
	}
	if err := setLog(); err != nil {
		return err
	}
	if err := setStorage(); err != nil {
		return err
	}

	if err := initLog(); err != nil {
		return err
	}

	return nil
}

func setServer() error {
	if v := os.Getenv("MAGENT_SERVER_ID"); v != "" {
		gConf.Server.ID = v
	} else {
		return errors.New("missing env: MAGENT_SERVER_ID")
	}

	if v := os.Getenv("MAGENT_SERVER_HOST"); v != "" {
		gConf.Server.Host = v
	}
	if v := os.Getenv("MAGENT_SERVER_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		gConf.Server.Port = port
	}

	if v := os.Getenv("MAGENT_SERVER_HEADER"); v != "" {
		gConf.Server.Header = v
	} else {
		return errors.New("missing env: MAGENT_SERVER_HEADER")
	}
	if v := os.Getenv("MAGENT_SERVER_SALT"); v != "" {
		gConf.Server.Salt = v
	} else {
		return errors.New("missing env: MAGENT_SERVER_SALT")
	}

	return nil
}

func setLog() error {
	if v := os.Getenv("MAGENT_LOG_PATH"); v != "" {
		gConf.Log.Path = v
	}
	if v := os.Getenv("MAGENT_LOG_LEVEL"); v != "" {
		gConf.Log.Level = v
	}
	if v := os.Getenv("MAGENT_LOG_SIZE"); v != "" {
		size, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		gConf.Log.Size = size
	}
	if v := os.Getenv("MAGENT_LOG_BACKUP"); v != "" {
		size, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		gConf.Log.Backup = size
	}

	return nil
}

func setStorage() error {
	if v := os.Getenv("MAGENT_STORAGE_DB"); v != "" {
		gConf.Store.DB = v
	} else {
		return errors.New("missing env: MAGENT_STORAGE_DB")
	}
	if v := os.Getenv("MAGENT_STORAGE_DSN"); v != "" {
		gConf.Store.Dsn = v
	} else {
		return errors.New("missing env: MAGENT_STORAGE_DSN")
	}

	if v := os.Getenv("MAGENT_STORAGE_MAXIDLECONN"); v != "" {
		size, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		gConf.Store.MaxIdleConn = size
	}
	if v := os.Getenv("MAGENT_STORAGE_MINIDLECONN"); v != "" {
		size, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		gConf.Store.MinIdleConn = size
	}
	if v := os.Getenv("MAGENT_STORAGE_IDLETIME"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return err
		}
		gConf.Store.MaxConnIdleTime = d
	}
	if v := os.Getenv("MAGENT_STORAGE_DEBUG"); v != "" {
		debug, err := strconv.ParseBool(v)
		if err != nil {
			return err
		}
		gConf.Store.Debug = debug
	}

	return nil
}

func initLog() error {
	if level, err := logrus.ParseLevel(gConf.Log.Level); err != nil {
		return err
	} else {
		logrus.SetLevel(level)
	}

	if gConf.Log.Path != "" {
		_ = os.MkdirAll(filepath.Dir(gConf.Log.Path), os.ModePerm)
		logrus.SetOutput(&lumberjack.Logger{
			Filename:   gConf.Log.Path,
			MaxSize:    gConf.Log.Size,
			MaxBackups: gConf.Log.Backup,
			LocalTime:  true,
			Compress:   true,
		})
	}

	f := &logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	}
	logrus.SetFormatter(f)

	return nil
}
