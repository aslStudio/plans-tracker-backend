package auth

import "context"

type contextKey int

const (
	userKey contextKey = iota
	tokenKey
)

func WithIdentity(ctx context.Context, userID, token string) context.Context {
	ctx = context.WithValue(ctx, userKey, userID)
	return context.WithValue(ctx, tokenKey, token)
}

func UserID(ctx context.Context) string {
	value, _ := ctx.Value(userKey).(string)
	return value
}

func AccessToken(ctx context.Context) string {
	value, _ := ctx.Value(tokenKey).(string)
	return value
}
