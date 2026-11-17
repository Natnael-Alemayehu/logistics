package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/natnael-alemayehu/kiflet/internal/db"
)

// APIService manages the keys of the public API (FR-046) and says whose a
// key is. What the API does, it does through the other services, as the
// dashboard does.
type APIService struct{ *core }

// APIKeyPrefix starts every key, so that one pasted in the wrong place can
// be recognised for what it is.
const APIKeyPrefix = "kf_"

// maxAPIKeys is how many working keys a company may hold.
const maxAPIKeys = 10

var errBadAPIKey = &Error{Kind: KindUnauthorized, Message: "That API key isn't valid. Send a working key as: Authorization: Bearer kf_…"}

func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// Keys lists a company's keys, working ones first.
func (s *APIService) Keys(ctx context.Context, tenantID pgtype.UUID) ([]db.ApiKey, error) {
	return s.q.ListAPIKeys(ctx, tenantID)
}

// CreateKey makes a key and returns it. It is not kept, only its hash, so
// this is the one time it can be read.
func (s *APIService) CreateKey(ctx context.Context, p *Principal, name string, ci ClientInfo) (string, db.ApiKey, error) {
	if !p.CanManageCompany() {
		return "", db.ApiKey{}, ErrUnauthorized
	}
	name = strings.Join(strings.Fields(name), " ")
	if n := utf8.RuneCountInString(name); n < 1 || n > 60 {
		return "", db.ApiKey{}, invalid("name", "Name the key after what will use it, in up to 60 characters.")
	}
	raw := make([]byte, 25)
	if _, err := rand.Read(raw); err != nil {
		return "", db.ApiKey{}, err
	}
	key := APIKeyPrefix + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))

	var out db.ApiKey
	err := s.tx(ctx, func(q *db.Queries, _ *afterCommit) error {
		// One at a time for a company, so that requests sent together
		// can't all pass the count.
		if _, err := q.GetTenantForUpdate(ctx, p.TenantID); err != nil {
			return err
		}
		n, err := q.CountActiveAPIKeys(ctx, p.TenantID)
		if err != nil {
			return err
		}
		if n >= maxAPIKeys {
			return limitReached("A company can hold 10 working keys. Revoke one that is no longer used.")
		}
		out, err = q.CreateAPIKey(ctx, db.CreateAPIKeyParams{
			TenantID: p.TenantID, Name: name, Prefix: key[:len(APIKeyPrefix)+6], KeyHash: hashAPIKey(key), CreatedBy: p.UserID,
		})
		if err != nil {
			return err
		}
		s.audit(ctx, q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "api_key_created", "api_key", out.ID, nil, map[string]any{"name": name, "prefix": out.Prefix})
		return nil
	})
	if err != nil {
		return "", db.ApiKey{}, err
	}
	return key, out, nil
}

// RevokeKey stops a key working, at once and for good.
func (s *APIService) RevokeKey(ctx context.Context, p *Principal, id pgtype.UUID, ci ClientInfo) error {
	if !p.CanManageCompany() {
		return ErrUnauthorized
	}
	k, err := s.q.RevokeAPIKey(ctx, db.RevokeAPIKeyParams{ID: id, TenantID: p.TenantID})
	if isNoRows(err) {
		return notFound("That key doesn't exist or is already revoked.")
	}
	if err != nil {
		return err
	}
	s.audit(ctx, s.q, p.Actor(SourceDashboard, ci.IP, ci.UserAgent), "api_key_revoked", "api_key", k.ID, map[string]any{"name": k.Name, "prefix": k.Prefix}, nil)
	return nil
}

// Authenticate finds whose a key is and returns the Actor its requests act
// as. Its name is the key's, so the history says which integration did
// what.
func (s *APIService) Authenticate(ctx context.Context, key string, ci ClientInfo) (Actor, error) {
	if !strings.HasPrefix(key, APIKeyPrefix) || len(key) < 20 {
		return Actor{}, errBadAPIKey
	}
	k, err := s.q.GetAPIKeyByHash(ctx, hashAPIKey(key))
	if isNoRows(err) {
		return Actor{}, errBadAPIKey
	}
	if err != nil {
		return Actor{}, err
	}
	if !k.TenantActive {
		return Actor{}, s.suspended()
	}
	if err := s.q.TouchAPIKey(ctx, k.ID); err != nil {
		s.log.Warn().Err(err).Msg("record an API key's use")
	}
	return Actor{TenantID: k.TenantID, Name: "API: " + k.Name, Source: SourceAPI, IP: ci.IP, UserAgent: ci.UserAgent}, nil
}
