package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"tls-rest/go/lib/db/rdb"
)

var SessionCacheInstance *Cache[Session]

func init() {
	SessionCacheInstance = NewCache(func(key string) (Session, error) {
		// Simulate an expensive operation
		dummy := Session{}
		redisClient, err := rdb.GetInstance()

		if err != nil {
			return dummy, err
		}

		ctx := context.Background()

		data, err := redisClient.Get(ctx, key).Result()
		if err == nil && data != "" {
			ci, err := DecodeJson(data)
			if err != nil {
				return dummy, err
			}

			return ci, nil
		}

		return dummy, err
	}, func(key string, value Session) error {
		// Simulate an expensive operation
		redisClient, err := rdb.GetInstance()
		if err != nil {
			return err
		}

		ctx := context.Background()

		data, err := value.EncodeJson()
		if err != nil {
			return err
		}

		duration := time.Until(value.Expire)

		return redisClient.Set(ctx, key, data, duration).Err()
	})
}

type Session struct {
	UserAgent  string
	IP         string
	UserID     int
	Username   string
	IsAdmin    bool
	UserRights map[int]int
	Expire     time.Time
	LastAccess time.Time
}

type Cache[T any] struct {
	sync.Mutex
	data       map[string]T
	getter     func(string) (T, error)
	setter     func(string, T) error
	lastAccess map[string]time.Time
	expireOn   map[string]time.Time
}

func (item *Session) EncodeJson() (string, error) {
	bytes, err := json.Marshal(item)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func DecodeJson(data string) (Session, error) {
	ci := Session{}
	err := json.Unmarshal([]byte(data), &ci)

	if err != nil {
		return Session{}, err
	}
	return ci, nil
}

func NewCache[T any](getter func(string) (T, error), setter func(string, T) error) *Cache[T] {
	return &Cache[T]{
		data:       make(map[string]T),
		getter:     getter,
		setter:     setter,
		lastAccess: make(map[string]time.Time),
		expireOn:   make(map[string]time.Time),
	}
}

func (c *Cache[T]) Set(key string, value T) {
	c.Lock()
	defer c.Unlock()

	c.data[key] = value
	if c.setter != nil {
		go c.setter(key, value)
	}
	c.lastAccess[key] = time.Now()
}

func (c *Cache[T]) Get(key string) (*T, error) {
	c.Lock()
	defer c.Unlock()

	dummy := new(T)

	if value, ok := c.data[key]; ok {

		if time.Now().Before(c.expireOn[key]) {
			c.lastAccess[key] = time.Now()
			return &value, nil
		}
		delete(c.data, key)
		delete(c.lastAccess, key)
		delete(c.expireOn, key)
	}

	value, err := c.getter(key)
	if err != nil {
		return dummy, err
	}

	c.data[key] = value
	return &value, nil
}

// func main() {
// 	expensiveOperation := func(key string) (string, error) {
// 		time.Sleep(1 * time.Second)
// 		return "value for " + key, nil
// 	}

// 	myCache := NewCache(expensiveOperation, 5*time.Second)

// 	val1, _ := myCache.Get("key1")
// 	fmt.Println(val1)

// 	val2, _ := myCache.Get("key1")
// 	fmt.Println(val2)

// 	time.Sleep(6 * time.Second)

// 	val3, _ := myCache.Get("key1")
// 	fmt.Println(val3)
// }
