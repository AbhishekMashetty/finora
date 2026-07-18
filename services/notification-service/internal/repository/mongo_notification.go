// Package repository implements domain.NotificationRepository against MongoDB.
package repository

import (
	"context"
	"time"

	"github.com/finora/notification-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const collectionName = "notifications"

// notificationDoc is the BSON-shaped record stored in Mongo. It mirrors
// domain.Notification but owns its own ID type (ObjectID) so the domain
// package never has to know about the mongo-driver.
type notificationDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"user_id"`
	Title     string             `bson:"title"`
	Message   string             `bson:"message"`
	Type      string             `bson:"type"`
	Read      bool               `bson:"read"`
	CreatedAt time.Time          `bson:"created_at"`
}

func (d notificationDoc) toDomain() domain.Notification {
	return domain.Notification{
		ID:        d.ID.Hex(),
		UserID:    d.UserID,
		Title:     d.Title,
		Message:   d.Message,
		Type:      d.Type,
		Read:      d.Read,
		CreatedAt: d.CreatedAt,
	}
}

// MongoNotificationRepository is the MongoDB-backed implementation of
// domain.NotificationRepository.
type MongoNotificationRepository struct {
	coll *mongo.Collection
}

// NewMongoNotificationRepository builds a repository against the given
// database's "notifications" collection.
func NewMongoNotificationRepository(db *mongo.Database) *MongoNotificationRepository {
	return &MongoNotificationRepository{coll: db.Collection(collectionName)}
}

// Create inserts a new notification and populates its generated ID.
func (r *MongoNotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	doc := notificationDoc{
		UserID:    n.UserID,
		Title:     n.Title,
		Message:   n.Message,
		Type:      n.Type,
		Read:      n.Read,
		CreatedAt: n.CreatedAt,
	}

	res, err := r.coll.InsertOne(ctx, doc)
	if err != nil {
		return err
	}

	oid, ok := res.InsertedID.(primitive.ObjectID)
	if ok {
		n.ID = oid.Hex()
	}
	return nil
}

// ListByUser returns one page of notifications owned by userID, newest
// first, optionally filtered to unread only. filter.Page/PageSize are
// assumed already defaulted/capped by the service layer — this method
// applies them as-is (skip/limit), matching expense-service's
// TransactionRepository.ListByUser convention exactly.
func (r *MongoNotificationRepository) ListByUser(ctx context.Context, userID string, filter domain.NotificationFilter) (domain.NotificationPage, error) {
	q := bson.M{"user_id": userID}
	if filter.UnreadOnly {
		q["read"] = false
	}

	total, err := r.coll.CountDocuments(ctx, q)
	if err != nil {
		return domain.NotificationPage{}, err
	}

	skip := int64(filter.Page-1) * int64(filter.PageSize)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(filter.PageSize))

	cursor, err := r.coll.Find(ctx, q, opts)
	if err != nil {
		return domain.NotificationPage{}, err
	}
	defer cursor.Close(ctx)

	notifications := make([]domain.Notification, 0)
	for cursor.Next(ctx) {
		var doc notificationDoc
		if err := cursor.Decode(&doc); err != nil {
			return domain.NotificationPage{}, err
		}
		notifications = append(notifications, doc.toDomain())
	}
	if err := cursor.Err(); err != nil {
		return domain.NotificationPage{}, err
	}
	return domain.NotificationPage{Notifications: notifications, Total: total}, nil
}

// MarkRead sets read=true on the notification identified by id, scoped to
// userID so a caller can never mark another user's notification as read.
func (r *MongoNotificationRepository) MarkRead(ctx context.Context, userID, id string) (*domain.Notification, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	filter := bson.M{"_id": oid, "user_id": userID}
	update := bson.M{"$set": bson.M{"read": true}}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc notificationDoc
	err = r.coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	result := doc.toDomain()
	return &result, nil
}
