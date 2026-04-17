package tokens

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenKind string

const (
	KindAccess  TokenKind = "access"
	KindRefresh TokenKind = "refresh"
)

type Claims struct {
	UserID uuid.UUID `json:"uid"`
	Role   string    `json:"role"`
	Kind   TokenKind `json:"kind"`
	jwt.RegisteredClaims
}

type Pair struct {
	Access           string
	Refresh          string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

type Issuer interface {
	Issue(userID uuid.UUID, role string) (Pair, error)
	Parse(token string) (*Claims, error)
}

type JWT struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
	issuer     string
	now        func() time.Time
}

type Option func(*JWT)

func WithClock(now func() time.Time) Option {
	return func(j *JWT) { j.now = now }
}

func NewJWT(secret string, accessTTL, refreshTTL time.Duration, issuer string, opts ...Option) *JWT {
	j := &JWT{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
		issuer:     issuer,
		now:        time.Now,
	}
	for _, opt := range opts {
		opt(j)
	}
	return j
}

func (j *JWT) Issue(userID uuid.UUID, role string) (Pair, error) {
	now := j.now()
	access, accessExp, err := j.sign(userID, role, KindAccess, now, j.accessTTL)
	if err != nil {
		return Pair{}, err
	}
	refresh, refreshExp, err := j.sign(userID, role, KindRefresh, now, j.refreshTTL)
	if err != nil {
		return Pair{}, err
	}
	return Pair{
		Access:           access,
		Refresh:          refresh,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
	}, nil
}

func (j *JWT) sign(userID uuid.UUID, role string, kind TokenKind, now time.Time, ttl time.Duration) (string, time.Time, error) {
	exp := now.Add(ttl)
	claims := Claims{
		UserID: userID,
		Role:   role,
		Kind:   kind,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        uuid.NewString(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(j.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("tokens: sign: %w", err)
	}
	return signed, exp, nil
}

var ErrInvalidToken = errors.New("invalid token")

func (j *JWT) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: unexpected signing method %v", ErrInvalidToken, t.Header["alg"])
		}
		return j.secret, nil
	}, jwt.WithTimeFunc(j.now))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
