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

type categoryDoc struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	UserID    string             `bson:"user_id"`
	Name      string             `bson:"name"`
	Type      string             `bson:"type"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

func (d categoryDoc) toDomain() domain.Category {
	return domain.Category{
		ID:        d.ID.Hex(),
		UserID:    d.UserID,
		Name:      d.Name,
		Type:      domain.TransactionType(d.Type),
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}

// CategoryRepository is the MongoDB-backed implementation of
// domain.CategoryRepository.
type CategoryRepository struct {
	col *mongo.Collection
}

// NewCategoryRepository builds a repository against the "categories" collection.
func NewCategoryRepository(db *mongo.Database) *CategoryRepository {
	return &CategoryRepository{col: db.Collection("categories")}
}

var _ domain.CategoryRepository = (*CategoryRepository)(nil)

func (r *CategoryRepository) Create(ctx context.Context, category *domain.Category) error {
	doc := categoryDoc{
		ID:        primitive.NewObjectID(),
		UserID:    category.UserID,
		Name:      category.Name,
		Type:      string(category.Type),
		CreatedAt: category.CreatedAt,
		UpdatedAt: category.UpdatedAt,
	}
	if _, err := r.col.InsertOne(ctx, doc); err != nil {
		return err
	}
	category.ID = doc.ID.Hex()
	return nil
}

func (r *CategoryRepository) ListByUser(ctx context.Context, userID string) ([]domain.Category, error) {
	cur, err := r.col.Find(ctx, bson.M{"user_id": userID}, options.Find().SetSort(bson.M{"created_at": -1}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	categories := make([]domain.Category, 0)
	for cur.Next(ctx) {
		var doc categoryDoc
		if err := cur.Decode(&doc); err != nil {
			return nil, err
		}
		categories = append(categories, doc.toDomain())
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return categories, nil
}
