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

// JTI returns the parsed token's unique ID as a UUID. It returns the zero
// UUID if the registered claim is missing or unparseable; callers that
// require a valid jti must check for uuid.Nil.
func (c *Claims) JTI() uuid.UUID {
	if c.ID == "" {
		return uuid.Nil
	}
	id, err := uuid.Parse(c.ID)
	if err != nil {
		return uuid.Nil
	}
	return id
}

type Pair struct {
	Access           string
	Refresh          string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
	// RefreshJTI is the unique identifier embedded in the refresh JWT. The
	// caller persists it so the token can be revoked or marked as used, which
	// is what makes refresh-token rotation and replay detection possible.
	RefreshJTI uuid.UUID
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
	accessJTI := uuid.New()
	refreshJTI := uuid.New()
	access, accessExp, err := j.sign(userID, role, KindAccess, accessJTI, now, j.accessTTL)
	if err != nil {
		return Pair{}, err
	}
	refresh, refreshExp, err := j.sign(userID, role, KindRefresh, refreshJTI, now, j.refreshTTL)
	if err != nil {
		return Pair{}, err
	}
	return Pair{
		Access:           access,
		Refresh:          refresh,
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
		RefreshJTI:       refreshJTI,
	}, nil
}

func (j *JWT) sign(userID uuid.UUID, role string, kind TokenKind, jti uuid.UUID, now time.Time, ttl time.Duration) (string, time.Time, error) {
	exp := now.Add(ttl)
	claims := Claims{
		UserID: userID,
		Role:   role,
		Kind:   kind,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    j.issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{j.issuer},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			ID:        jti.String(),
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
	},
		jwt.WithTimeFunc(j.now),
		jwt.WithIssuer(j.issuer),
		jwt.WithAudience(j.issuer),
		jwt.WithLeeway(30*time.Second),
		jwt.WithExpirationRequired(),
		jwt.WithValidMethods([]string{"HS256"}),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
