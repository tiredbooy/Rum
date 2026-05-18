package queue

import "time"

type Download struct {
	ID        string     `bson:"_id"`
	URL       string     `bson:"url"`
	Status    string     `bson:"status"`
	StartTime time.Time  `bson:"start_time"`
	EndTime   *time.Time `bson:"end_time"`
}

// type Queue struct {
// 	db *
// }
