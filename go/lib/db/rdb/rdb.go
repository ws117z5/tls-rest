package rdb

import (
	"context"

	config "tls-rest/go/constants"

	redis "github.com/go-redis/redis/v8"
)

// Nil proto.Nil
const Nil = redis.Nil

// GetInstance returns *redis.Client
func GetInstance() (*redis.Client, error) {
	//TODO what else will there be in there? DO we need more DB instances?
	//

	db := redis.NewClient(&redis.Options{
		Addr:     config.RDb.Addr,
		Password: config.RDb.Password,
		DB:       config.RDb.DatabaseInt, // use default DB
	})

	ctx := context.Background()
	_, err := db.Ping(ctx).Result()

	return db, err
}
