package config

import "time"

type Config struct {
	Server  *ServerConfig
	Log     *LogConfig
	Store   *StoreConfig
	Runtime *RuntimeConfig
}

type ServerConfig struct {
	Host   string // 服务地址
	Port   int    // 监听端口
	Header string // header
	Salt   string // token 密钥
}

type LogConfig struct {
	Path   string // 文件位置
	Level  string // 日志等级
	Size   int    // 日志大小限制
	Backup int    // 最大备份数量
}

type StoreConfig struct {
	DB              string
	Dsn             string
	MaxIdleConn     int
	MinIdleConn     int
	MaxConnIdleTime time.Duration
	Debug           bool
}

type RuntimeConfig struct {
	Token string
	Topic TopicConfig
	Image string
}

type TopicConfig struct {
	Url    string
	ApiKey string
	Model  string
}
