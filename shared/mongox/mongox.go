// Package mongox provides the one way Finora services connect to MongoDB and
// expose that connection as a health.Checker, so every service's /ready
// behaves identically.
package mongox

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Connect dials uri with a bounded timeout and verifies connectivity with a ping.
func Connect(uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, err
	}
	return client, nil
}

// Checker adapts a *mongo.Client into a health.Checker via Ping.
type Checker struct {
	Client *mongo.Client
}

func (c Checker) Name() string { return "mongodb" }

func (c Checker) Check(ctx context.Context) error {
	return c.Client.Ping(ctx, readpref.Primary())
}
