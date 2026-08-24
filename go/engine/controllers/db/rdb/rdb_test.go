package rdb_test

import (
	"context"
	"testing"
	"time"

	"tls-rest/go/engine/controllers/db/rdb"

	"github.com/go-redis/redis/v8"
)

func TestRedis(t *testing.T) {

	ctx := context.Background()

	db, err := rdb.GetInstance()

	if err != nil {
		t.Error(err)
	}

	err = db.Set(ctx, "key", "value", 0).Err()
	if err != nil {
		t.Error(err)
	}

	val, err := db.Get(ctx, "key").Result()
	if err != nil {
		t.Error(err)
	}
	t.Log("key", val)

	val2, err := db.Get(ctx, "missing_key").Result()
	if err == redis.Nil {
		t.Log("missing_key does not exist")
	} else if err != nil {
		t.Error(err)
	} else {
		t.Log("missing_key", val2)
	}

	if err := db.RPush(ctx, "queue", "message").Err(); err != nil {
		t.Error(err)
	}

	// use `redisdb.BLPop(0, "queue")` for infinite waiting time
	result, err := db.BLPop(ctx, 1*time.Second, "queue").Result()
	if err != nil {
		t.Error(err)
	}

	t.Log(result[0], result[1])

	err = db.Set(ctx, "key", "value", 0).Err()
	if err != nil {
		t.Error(err)
	}

	val, err = db.Get(ctx, "key").Result()
	if err != nil {
		t.Error(err)
	}
	t.Log("key", val)

	val2, err = db.Get(ctx, "key2").Result()
	if err == redis.Nil {
		t.Log("key2 does not exist")
	} else if err != nil {
		t.Error(err)
	} else {
		t.Log(val2)
	}

	set, err := db.SetNX(ctx, "key", "value", 10*time.Second).Result()
	if err != nil {
		t.Error(err)
	}
	t.Log(set)
	// SORT list LIMIT 0 2 ASC
	vals, err := db.Sort(ctx, "list", &redis.Sort{Offset: 0, Count: 2, Order: "ASC"}).Result()

	if err != nil {
		t.Error(err)
	} else {
		t.Log(vals)
	}
	// ZRANGEBYSCORE zset -inf +inf WITHSCORES LIMIT 0 2
	valsZ, err := db.ZRangeByScoreWithScores(ctx, "zset", &redis.ZRangeBy{
		Min:    "-inf",
		Max:    "+inf",
		Offset: 0,
		Count:  2,
	}).Result()
	if err != nil {
		t.Error(err)
	} else {
		t.Log(valsZ)
	}
	// ZINTERSTORE out 2 zset1 zset2 WEIGHTS 2 3 AGGREGATE SUM
	valsZSt, err := db.ZInterStore(ctx, "out", &redis.ZStore{
		Weights: []float64{2, 3},
		Keys:    []string{"zset1", "zset2"},
	}).Result()
	if err != nil {
		t.Error(err)
	} else {
		t.Log(valsZSt)
	}
	// EVAL "return {KEYS[1],ARGV[1]}" 1 "key" "hello"
	vals3, err := db.Eval(ctx, "return {KEYS[1],ARGV[1]}", []string{"key"}, "hello").Result()
	if err != nil {
		t.Error(err)
	} else {
		t.Log(vals3)
	}
}
