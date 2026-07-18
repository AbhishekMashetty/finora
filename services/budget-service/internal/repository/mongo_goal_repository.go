package repository

import (
	"context"
	"errors"
	"time"

	"github.com/finora/budget-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// goalDocument is the BSON shape stored in MongoDB for a savings goal.
type goalDocument struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	UserID        string             `bson:"user_id"`
	Name          string             `bson:"name"`
	TargetAmount  float64            `bson:"target_amount"`
	TargetDate    time.Time          `bson:"target_date"`
	CurrentAmount float64            `bson:"current_amount"`
	CreatedAt     time.Time          `bson:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at"`
}

func (d goalDocument) toDomain() domain.Goal {
	return domain.Goal{
		ID:            d.ID.Hex(),
		UserID:        d.UserID,
		Name:          d.Name,
		TargetAmount:  d.TargetAmount,
		TargetDate:    d.TargetDate,
		CurrentAmount: d.CurrentAmount,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

// MongoGoalRepository implements domain.GoalRepository against a Mongo
// collection.
type MongoGoalRepository struct {
	collection *mongo.Collection
}

// NewMongoGoalRepository builds a repository against the "goals" collection
// in db.
func NewMongoGoalRepository(db *mongo.Database) *MongoGoalRepository {
	return &MongoGoalRepository{collection: db.Collection("goals")}
}

func (r *MongoGoalRepository) Create(ctx context.Context, goal *domain.Goal) error {
	now := time.Now().UTC()
	doc := goalDocument{
		ID:            primitive.NewObjectID(),
		UserID:        goal.UserID,
		Name:          goal.Name,
		TargetAmount:  goal.TargetAmount,
		TargetDate:    goal.TargetDate,
		CurrentAmount: 0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		return err
	}

	*goal = doc.toDomain()
	return nil
}

func (r *MongoGoalRepository) ListByUser(ctx context.Context, userID string) ([]domain.Goal, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	goals := make([]domain.Goal, 0)
	for cursor.Next(ctx) {
		var doc goalDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		goals = append(goals, doc.toDomain())
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return goals, nil
}

func (r *MongoGoalRepository) GetByIDForUser(ctx context.Context, id, userID string) (*domain.Goal, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	var doc goalDocument
	err = r.collection.FindOne(ctx, bson.M{"_id": oid, "user_id": userID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	goal := doc.toDomain()
	return &goal, nil
}

func (r *MongoGoalRepository) Update(ctx context.Context, goal *domain.Goal) error {
	oid, err := primitive.ObjectIDFromHex(goal.ID)
	if err != nil {
		return domain.ErrNotFound
	}

	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"name":           goal.Name,
			"target_amount":  goal.TargetAmount,
			"target_date":    goal.TargetDate,
			"current_amount": goal.CurrentAmount,
			"updated_at":     now,
		},
	}

	var doc goalDocument
	err = r.collection.FindOneAndUpdate(
		ctx,
		bson.M{"_id": oid, "user_id": goal.UserID},
		update,
	).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.ErrNotFound
		}
		return err
	}

	goal.UpdatedAt = now
	return nil
}

func (r *MongoGoalRepository) DeleteByIDForUser(ctx context.Context, id, userID string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrNotFound
	}

	res, err := r.collection.DeleteOne(ctx, bson.M{"_id": oid, "user_id": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
