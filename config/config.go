package config

import (
	"ProjectMessageService/internal/loggers"
	"os"

	"github.com/gookit/slog"
	"github.com/spf13/viper"
)

type Application struct {
	Log *slog.Logger
}

func SetupApplication() *Application {
	// Настройка логгера перед его инициализацией
	loggers.SetLogConsole(true) // Логи будут записываться в файл
	loggers.SetIsDebugMode(true)
	loggers.SetIsInfoMode(true)
	loggers.SetIsWarnMode(true)
	// Настраиваем логгер
	logger := loggers.SetupLogger()
	// Создаем экземпляр Application с настроенным логгером
	return &Application{Log: logger}
}

type Config struct {
	DBHost     string `mapstructure:"DB_HOST"`
	DBPort     string `mapstructure:"DB_PORT"`
	DBUser     string `mapstructure:"DB_USER"`
	DBPassword string `mapstructure:"DB_PASSWORD"`
	DBName     string `mapstructure:"DB_NAME"`
	KafkaURL   string `mapstructure:"KAFKA_URL"`
}

func LoadConfig(app *Application) (cfg Config) {
	// Чтение файла app.env
	viper.AddConfigPath(".")
	viper.SetConfigName("app")
	viper.SetConfigType("env")

	// Чтение переменных окружения
	viper.AutomaticEnv() // Автоматически читать переменные окружения

	if err := viper.MergeInConfig(); err != nil {
		app.Log.Printf("Error reading app.env file, %s", err)
	}

	if err := viper.ReadInConfig(); err != nil {
		app.Log.Fatalf("Error reading config file, %s", err)
	}
	/*
		cfg := Config{
			DBHost:     viper.GetString("DB_HOST"),
			DBPort:     viper.GetString("DB_PORT"),
			DBUser:     viper.GetString("DB_USER"),
			DBPassword: viper.GetString("DB_PASSWORD"),
			DBName:     viper.GetString("DB_NAME"),
			KafkaURL:   viper.GetString("KAFKA_URL"),
		}

	*/
	err := viper.Unmarshal(&cfg)
	if err != nil {
		app.Log.Fatalf("unable to decode into struct, %v", err)
	}

	// Проверка, запущено ли приложение внутри Docker
	if _, err = os.Stat("/.dockerenv"); err == nil {
		// Внутри Docker
		cfg.KafkaURL = "kafka:9092"
	} else {
		// Вне Docker
		cfg.KafkaURL = "localhost:29092"
	}

	return cfg
}
