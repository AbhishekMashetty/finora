// Package repository implements domain repository interfaces against
// MongoDB. Handlers and services never import mongo-driver directly — only
// this package does.
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

// budgetDocument is the BSON shape stored in MongoDB. It mirrors
// domain.Budget but uses primitive.ObjectID for _id, which the domain layer
// never has to know about.
type budgetDocument struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"user_id"`
	Category  string             `bson:"category"`
	Amount    float64            `bson:"amount"`
	Period    string             `bson:"period"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`

	// LastNotifiedAt mirrors domain.Budget.LastNotifiedAt — Phase 4 dedup
	// bookkeeping for the overspend notification trigger in
	// internal/service/report_service.go. omitempty so pre-Phase-4 budgets
	// (and freshly created ones) simply omit the field rather than storing
	// an explicit null.
	LastNotifiedAt *time.Time `bson:"last_notified_at,omitempty"`
}

func (d budgetDocument) toDomain() domain.Budget {
	return domain.Budget{
		ID:             d.ID.Hex(),
		UserID:         d.UserID,
		Category:       d.Category,
		Amount:         d.Amount,
		Period:         domain.Period(d.Period),
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
		LastNotifiedAt: d.LastNotifiedAt,
	}
}

// MongoBudgetRepository implements domain.BudgetRepository against a Mongo
// collection.
type MongoBudgetRepository struct {
	collection *mongo.Collection
}

// NewMongoBudgetRepository builds a repository against the "budgets"
// collection in db.
func NewMongoBudgetRepository(db *mongo.Database) *MongoBudgetRepository {
	return &MongoBudgetRepository{collection: db.Collection("budgets")}
}

func (r *MongoBudgetRepository) Create(ctx context.Context, budget *domain.Budget) error {
	now := time.Now().UTC()
	doc := budgetDocument{
		ID:        primitive.NewObjectID(),
		UserID:    budget.UserID,
		Category:  budget.Category,
		Amount:    budget.Amount,
		Period:    string(budget.Period),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err := r.collection.InsertOne(ctx, doc); err != nil {
		return err
	}

	*budget = doc.toDomain()
	return nil
}

func (r *MongoBudgetRepository) ListByUser(ctx context.Context, userID string) ([]domain.Budget, error) {
	cursor, err := r.collection.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	budgets := make([]domain.Budget, 0)
	for cursor.Next(ctx) {
		var doc budgetDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		budgets = append(budgets, doc.toDomain())
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return budgets, nil
}

func (r *MongoBudgetRepository) GetByIDForUser(ctx context.Context, id, userID string) (*domain.Budget, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}

	var doc budgetDocument
	err = r.collection.FindOne(ctx, bson.M{"_id": oid, "user_id": userID}).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	budget := doc.toDomain()
	return &budget, nil
}

func (r *MongoBudgetRepository) Update(ctx context.Context, budget *domain.Budget) error {
	oid, err := primitive.ObjectIDFromHex(budget.ID)
	if err != nil {
		return domain.ErrNotFound
	}

	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"category":         budget.Category,
			"amount":           budget.Amount,
			"period":           string(budget.Period),
			"updated_at":       now,
			"last_notified_at": budget.LastNotifiedAt,
		},
	}

	var doc budgetDocument
	err = r.collection.FindOneAndUpdate(
		ctx,
		bson.M{"_id": oid, "user_id": budget.UserID},
		update,
	).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return domain.ErrNotFound
		}
		return err
	}

	budget.UpdatedAt = now
	return nil
}

func (r *MongoBudgetRepository) DeleteByIDForUser(ctx context.Context, id, userID string) error {
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
