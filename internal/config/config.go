package config

// этот конфиг файл сопоставляется yaml файлу

import (
	"log"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

// struct tags
type Config struct {
	Env        string `yaml:"env" env-default:"local"`         // при сериализации/десериализации в YAML поле Env будет соответствовать ключу env.Без этого тега использовалось бы имя поля (Env)
	StorageDSN string `yaml:"storage_dsn" env-required:"true"` // <-- ИЗМЕНИЛОСЬ ЗДЕСЬ
	HTTPServer `yaml:"http_server"`
}

//type HTTPUser struct {
//	User     string `yaml:"user" env-required:"true"`
//	Password string `yaml:"password" env-required:"true" env:"HTTP_SERVER_PASSWORD"`
//}
//
//type HTTPServer struct {
//	Address     string        `yaml:"address" env-default:"localhost:8080"`
//	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
//	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
//	Users       []HTTPUser    `yaml:"users"`
//}

type HTTPServer struct {
	Address     string        `yaml:"address" env-default:"localhost:8080"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
	User        string        `yaml:"user" env-required:"true"`
	Password    string        `yaml:"password" env-required:"true" env:"HTTP_SERVER_PASSWORD"`
}

// ф-я, котор прочитает йамл и создаст и заполнит конфиг
// В ее названии "MustLoad" - маст значит, Что в случае ошибки функция будет паниковать. Здесь это уместно, ибо конфиг - это архиважно. Приложение только запускается и его не страшно уронить
func MustLoad() *Config {
	configPath := os.Getenv("CONFIG_PATH") // путь К конфигу будем брать из переменной окружения
	if configPath == "" {
		log.Fatal("CONFIG_PATH is not set")
	}

	// check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config: %s", err)
	}

	if err := cleanenv.UpdateEnv(&cfg); err != nil {
		log.Fatalf("cannot update config from env: %s", err)
	}

	return &cfg
}
