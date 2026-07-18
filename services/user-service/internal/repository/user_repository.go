package repository

import (
	"context"
	"errors"

	"github.com/finora/user-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type userRepository struct {
	col *mongo.Collection
}

// NewUserRepository returns a MongoDB-backed domain.UserRepository.
func NewUserRepository(db *mongo.Database) domain.UserRepository {
	return &userRepository{col: db.Collection(usersCollection)}
}

func (r *userRepository) Create(ctx context.Context, u *domain.User) error {
	_, err := r.col.InsertOne(ctx, u)
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrDuplicateEmail
	}
	return err
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	err := r.col.FindOne(ctx, bson.M{"email": email}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var u domain.User
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *userRepository) Update(ctx context.Context, u *domain.User) error {
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": u.ID}, u)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}
