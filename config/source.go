package config

type Source interface {
	Start()
	Stop()
	Watch(func(*Config, error))
}
