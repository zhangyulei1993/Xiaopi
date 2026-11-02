package httpapi

import "context"

type claimsContextKey struct{}

func withClaims(ctx context.Context, claims tokenClaims) context.Context {
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

func claimsFromContext(ctx context.Context) tokenClaims {
	claims, _ := ctx.Value(claimsContextKey{}).(tokenClaims)
	return claims
}
