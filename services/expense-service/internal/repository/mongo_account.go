// Package repository contains MongoDB implementations of the domain
// repository interfaces. Handlers and services never import this package
// directly for its concrete types — they depend on domain interfaces so the
// service layer stays unit-testable with fakes.
package repository

import (
	"context"
	"time"

	"github.com/finora/expense-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// accountDoc is the BSON-mapped shape stored in Mongo. Kept separate from
// domain.Account so the domain package never has to know about
// primitive.ObjectID or bson tags.
type accountDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"user_id"`
	Name      string             `bson:"name"`
	Type      string             `bson:"type"`
	Currency  string             `bson:"currency"`
	Balance   float64            `bson:"balance"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

func (d accountDoc) toDomain() domain.Account {
	return domain.Account{
		ID:        d.ID.Hex(),
		UserID:    d.UserID,
		Name:      d.Name,
		Type:      domain.AccountType(d.Type),
		Currency:  d.Currency,
		Balance:   d.Balance,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// AccountRepository is the MongoDB-backed implementation of
// domain.AccountRepository.
type AccountRepository struct {
	col *mongo.Collection
}

// NewAccountRepository builds a repository against the "accounts" collection.
func NewAccountRepository(db *mongo.Database) *AccountRepository {
	return &AccountRepository{col: db.Collection("accounts")}
}

var _ domain.AccountRepository = (*AccountRepository)(nil)

func (r *AccountRepository) Create(ctx context.Context, account *domain.Account) error {
	doc := accountDoc{
		ID:        primitive.NewObjectID(),
		UserID:    account.UserID,
		Name:      account.Name,
		Type:      string(account.Type),
		Currency:  account.Currency,
		Balance:   account.Balance,
		CreatedAt: account.CreatedAt,
		UpdatedAt: account.UpdatedAt,
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return err
	}
	account.ID = doc.ID.Hex()
	return nil
}

func (r *AccountRepository) ListByUser(ctx context.Context, userID string) ([]domain.Account, error) {
	cur, err := r.col.Find(ctx, bson.M{"user_id": userID}, options.Find().SetSort(bson.M{"created_at": -1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	accounts := make([]domain.Account, 0)
	for cur.Next(ctx) {
		var doc accountDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		accounts = append(accounts, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *AccountRepository) GetByIDForUser(ctx context.Context, id, userID string) (*domain.Account, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	var doc accountDoc
	err = r.col.FindOne(ctx, bson.M{"_id": oid, "user_id": userID}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	account := doc.toDomain()
	return &account, nil
}

func (r *AccountRepository) Update(ctx context.Context, account *domain.Account) error {
	oid, err := primitive.ObjectIDFromHex(account.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	update := bson.M{
		"$set": bson.M{
			"name":       account.Name,
			"type":       string(account.Type),
			"currency":   account.Currency,
			"balance":    account.Balance,
			"updated_at": account.UpdatedAt,
		},
	}
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": oid, "user_id": account.UserID}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AccountRepository) DeleteByIDForUser(ctx context.Context, id, userID string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return domain.ErrNotFound
	}
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": oid, "user_id": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
