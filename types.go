package main

type SecretConfig struct {
	Path     string `yaml:"path"`
	Key      string `yaml:"key"`
	Variable string `yaml:"variable"`
}
