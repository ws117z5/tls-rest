package rdb

import (
	"context"

	config "tls-rest/go/constants"
	"tls-rest/go/engine/controllers/log"

	redis "github.com/go-redis/redis/v8"
)

// Nil proto.Nil
const Nil = redis.Nil

// GetInstance returns *redis.Client
func GetInstance() (*redis.Client, error) {
	//TODO what else will there be in there? DO we need more DB instances?
	//

	logger := log.For("redis")
	db := redis.NewClient(&redis.Options{
		Addr:     config.RDb.Addr,
		Password: config.RDb.Password,
		DB:       config.RDb.DatabaseInt, // use default DB
	})

	//logger.Debugf("connecting to %s (db %d)", config.RDb.Addr, config.RDb.DatabaseInt)
	ctx := context.Background()
	_, err := db.Ping(ctx).Result()
	if err != nil {
		logger.Errorf("connect to %s failed: %v", config.RDb.Addr, err)
	} else {
		//logger.Debugf("connected to %s", config.RDb.Addr)
	}

	return db, err
}
