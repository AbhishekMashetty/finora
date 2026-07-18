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

type transactionDoc struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	UserID     string             `bson:"user_id"`
	AccountID  string             `bson:"account_id"`
	CategoryID *string            `bson:"category_id,omitempty"`
	Type       string             `bson:"type"`
	Amount     float64            `bson:"amount"`
	Currency   string             `bson:"currency"`
	Date       time.Time          `bson:"date"`
	Note       string             `bson:"note,omitempty"`
	CreatedAt  time.Time          `bson:"created_at"`
	UpdatedAt  time.Time          `bson:"updated_at"`
}

func (d transactionDoc) toDomain() domain.Transaction {
	return domain.Transaction{
		ID:         d.ID.Hex(),
		UserID:     d.UserID,
		AccountID:  d.AccountID,
		CategoryID: d.CategoryID,
		Type:       domain.TransactionType(d.Type),
		Amount:     d.Amount,
		Currency:   d.Currency,
		Date:       d.Date,
		Note:       d.Note,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}

// TransactionRepository is the MongoDB-backed implementation of
// domain.TransactionRepository.
type TransactionRepository struct {
	col *mongo.Collection
}

// NewTransactionRepository builds a repository against the "transactions" collection.
func NewTransactionRepository(db *mongo.Database) *TransactionRepository {
	return &TransactionRepository{col: db.Collection("transactions")}
}

var _ domain.TransactionRepository = (*TransactionRepository)(nil)

func (r *TransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	doc := transactionDoc{
		ID:         primitive.NewObjectID(),
		UserID:     tx.UserID,
		AccountID:  tx.AccountID,
		CategoryID: tx.CategoryID,
		Type:       string(tx.Type),
		Amount:     tx.Amount,
		Currency:   tx.Currency,
		Date:       tx.Date,
		Note:       tx.Note,
		CreatedAt:  tx.CreatedAt,
		UpdatedAt:  tx.UpdatedAt,
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return err
	}
	tx.ID = doc.ID.Hex()
	return nil
}

func buildTransactionFilter(userID string, filter domain.TransactionFilter) bson.M {
	q := bson.M{"user_id": userID}
	if filter.AccountID != "" {
		q["account_id"] = filter.AccountID
	}
	if filter.Category != "" {
		q["category_id"] = filter.Category
	}
	if filter.From != nil || filter.To != nil {
		dateFilter := bson.M{}
		if filter.From != nil {
			dateFilter["$gte"] = *filter.From
		}
		if filter.To != nil {
			dateFilter["$lte"] = *filter.To
		}
		q["date"] = dateFilter
	}
	return q
}

func (r *TransactionRepository) ListByUser(ctx context.Context, userID string, filter domain.TransactionFilter) (domain.TransactionPage, error) {
	q := buildTransactionFilter(userID, filter)

	total, err := r.col.CountDocuments(ctx, q)
	if err != nil {
		return domain.TransactionPage{}, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	skip := int64(page-1) * int64(pageSize)

	opts := options.Find().
		SetSort(bson.M{"date": -1}).
		SetSkip(skip).
		SetLimit(int64(pageSize))

	cur, err := r.col.Find(ctx, q, opts)
	if err != nil {
		return domain.TransactionPage{}, err
	}
	defer cur.Close(ctx)

	transactions := make([]domain.Transaction, 0)
	for cur.Next(ctx) {
		var doc transactionDoc
		if err := cur.Decode(&doc); err != nil {
			return domain.TransactionPage{}, err
		}
		transactions = append(transactions, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return domain.TransactionPage{}, err
	}

	return domain.TransactionPage{Transactions: transactions, Total: total}, nil
}

func (r *TransactionRepository) GetByIDForUser(ctx context.Context, id, userID string) (*domain.Transaction, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, domain.ErrNotFound
	}
	var doc transactionDoc
	err = r.col.FindOne(ctx, bson.M{"_id": oid, "user_id": userID}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	tx := doc.toDomain()
	return &tx, nil
}

func (r *TransactionRepository) Update(ctx context.Context, tx *domain.Transaction) error {
	oid, err := primitive.ObjectIDFromHex(tx.ID)
	if err != nil {
		return domain.ErrNotFound
	}
	update := bson.M{
		"$set": bson.M{
			"account_id":  tx.AccountID,
			"category_id": tx.CategoryID,
			"type":        string(tx.Type),
			"amount":      tx.Amount,
			"currency":    tx.Currency,
			"date":        tx.Date,
			"note":        tx.Note,
			"updated_at":  tx.UpdatedAt,
		},
	}
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": oid, "user_id": tx.UserID}, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *TransactionRepository) DeleteByIDForUser(ctx context.Context, id, userID string) error {
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
