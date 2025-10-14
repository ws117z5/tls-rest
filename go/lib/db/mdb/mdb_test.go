package mdb_test

import "testing"

//"context"
//"log"
//"testing"
//"time"
//config "tls-rest/go/constants"
//"tls-rest/go/lib/db/mdb"

//"labix.org/v2/mgo/bson"

func TestMongo(t *testing.T) {
	// db, _ := mdb.GetInstance()
	// if ctx, err := context.WithTimeout(context.Background(), time.Duration(config.MDb.Timeout)*time.Second); err != nil {
	// 	log.Fatal(err)
	// } else {
	// 	collection := db.Collection("numbers")

	// 	cur, err := collection.Find(ctx, nil)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	defer cur.Close(ctx)

	// 	for cur.Next(ctx) {
	// 		var result bson.M
	// 		err := cur.Decode(&result)
	// 		if err != nil {
	// 			log.Fatal(err)
	// 		}
	// 		// do something with result....
	// 	}
	// 	if err := cur.Err(); err != nil {
	// 		log.Fatal(err)
	// 	}
	// }

}
