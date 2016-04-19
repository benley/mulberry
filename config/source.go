package config

type Source interface {
	Close()
	Watch(func(*Config, error))
}
