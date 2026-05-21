package settings

import (
	"github.com/spf13/viper"
)

func Init() error {
	viper.SetConfigFile("config.yaml")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	return nil
}
