package repository

import (
	"context"
	"errors"

	"github.com/finora/user-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type settingsRepository struct {
	col *mongo.Collection
}

// NewSettingsRepository returns a MongoDB-backed domain.SettingsRepository.
func NewSettingsRepository(db *mongo.Database) domain.SettingsRepository {
	return &settingsRepository{col: db.Collection(settingsCollection)}
}

func (r *settingsRepository) FindByUserID(ctx context.Context, userID string) (*domain.Settings, error) {
	var s domain.Settings
	err := r.col.FindOne(ctx, bson.M{"user_id": userID}).Decode(&s)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *settingsRepository) Upsert(ctx context.Context, s *domain.Settings) error {
	opts := options.Replace().SetUpsert(true)
	_, err := r.col.ReplaceOne(ctx, bson.M{"user_id": s.UserID}, s, opts)
	return err
}
