package middleware

import "context"

// PrincipalSlot is a mutable container for principal information set by
// Authn and read by RequestLogger. It exists because the outermost
// RequestLogger can't see the principal that an inner Authn writes via
// `r.WithContext(...)` — that's a child context, invisible to the parent.
//
// The slot solves this with a pointer: the outer middleware puts a fresh
// *PrincipalSlot in the context up front; Authn writes into it; the outer
// middleware reads after next.ServeHTTP returns.
type PrincipalSlot struct {
	Principal Principal
	Set       bool
}

type principalSlotKey struct{}

// WithPrincipalSlot returns a context carrying slot. RequestLogger calls
// this before invoking next.ServeHTTP.
func WithPrincipalSlot(ctx context.Context, slot *PrincipalSlot) context.Context {
	return context.WithValue(ctx, principalSlotKey{}, slot)
}

// PrincipalSlotFrom returns the slot pointer, or nil if WithPrincipalSlot
// was never called for this request. Returning nil (not a zero struct) lets
// callers distinguish "no slot configured" from "slot present but empty".
func PrincipalSlotFrom(ctx context.Context) *PrincipalSlot {
	slot, _ := ctx.Value(principalSlotKey{}).(*PrincipalSlot)
	return slot
}
