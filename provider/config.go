package provider

import (
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type (
	Config struct {
		TelegramBotConfig TelegramBotConfig `yaml:"telegram-bot"`
		LoggerConfig      LoggerConfig      `yaml:"logger"`
	}

	TelegramBotConfig struct {
		BingU  string `yaml:"bing-u"`
		ApiKey string `yaml:"api-key"`

		FallbackAnswer string `yaml:"callback-answer"`
		RepeatedAnswer string `yaml:"repeated-answer"`

		CommandResetAnswer string `yaml:"command-reset-answer"`
		CommandStartAnswer string `yaml:"command-start-answer"`
		CommandHelpAnswer  string `yaml:"command-help-answer"`

		UseProxy  bool   `yaml:"use-proxy"`
		HttpProxy string `yaml:"http-proxy"`
	}

	LoggerConfig struct {
		Level      string `yaml:"level"`
		Filename   string `yaml:"filename"`
		MaxSize    int    `yaml:"max-size"`
		MaxAge     int    `yaml:"max-age"`
		MaxBackups int    `yaml:"max-backups"`
		LocalTime  bool   `yaml:"local-time"`
		Compress   bool   `yaml:"compress"`
	}
)

func LoadConfig() (*Config, error) {
	viper.SetConfigName("sydney")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/sydney")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.SetDefault("logger.level", "debug")
	viper.SetDefault("logger.filename", "logs/sydney.log")
	viper.SetDefault("logger.max-size", 100)
	viper.SetDefault("logger.max-age", 30)
	viper.SetDefault("logger.max-backups", 10)
	viper.SetDefault("logger.compress", true)

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = viper.Unmarshal(&cfg, func(decoderConfig *mapstructure.DecoderConfig) {
		decoderConfig.TagName = "yaml"
	})
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
