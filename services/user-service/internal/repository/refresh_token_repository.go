package repository

import (
	"context"
	"errors"

	"github.com/finora/user-service/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type refreshTokenRepository struct {
	col *mongo.Collection
}

// NewRefreshTokenRepository returns a MongoDB-backed domain.RefreshTokenRepository.
func NewRefreshTokenRepository(db *mongo.Database) domain.RefreshTokenRepository {
	return &refreshTokenRepository{col: db.Collection(tokensCollection)}
}

func (r *refreshTokenRepository) Create(ctx context.Context, rt *domain.RefreshToken) error {
	_, err := r.col.InsertOne(ctx, rt)
	return err
}

func (r *refreshTokenRepository) FindByJTIHash(ctx context.Context, jtiHash string) (*domain.RefreshToken, error) {
	var rt domain.RefreshToken
	err := r.col.FindOne(ctx, bson.M{"jti_hash": jtiHash}).Decode(&rt)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &rt, nil
}

func (r *refreshTokenRepository) Revoke(ctx context.Context, id string) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"revoked": true}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// RevokeAllForUser revokes every non-revoked refresh token for userID in a
// single update-many. Unlike Revoke, a no-op match (no non-revoked tokens
// left) is not an error — this is a blast-radius containment action, not a
// lookup, so "nothing left to revoke" is a legitimate outcome.
func (r *refreshTokenRepository) RevokeAllForUser(ctx context.Context, userID string) error {
	_, err := r.col.UpdateMany(ctx,
		bson.M{"user_id": userID, "revoked": false},
		bson.M{"$set": bson.M{"revoked": true}},
	)
	return err
}
