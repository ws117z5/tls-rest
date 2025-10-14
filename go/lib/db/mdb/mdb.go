package mdb

import (
	"github.com/ws117z5/tls-rest/go/constants"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetInstance retturns MDb *mongo.Client
func GetInstance() (*mongo.Client, error) {
	return mongo.NewClient(options.Client().ApplyURI(constants.MDb.Addr))

	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	//err = client.Connect(ctx)

	//return mongo.NewClient(config.MDb.Addr)
}
