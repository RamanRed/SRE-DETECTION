package postgres

import (
	"context"
	"time"

	"github.com/RamanRed/SRE-DETECTION/apps/incident-service/internal/secretbox"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool    *pgxpool.Pool
	secrets secretbox.Codec
}

func New(pool *pgxpool.Pool, codecs ...secretbox.Codec) *Store {
	defaultCodec, _ := secretbox.New("")
	var codec secretbox.Codec = defaultCodec
	if len(codecs) > 0 && codecs[0] != nil {
		codec = codecs[0]
	}
	return &Store{pool: pool, secrets: codec}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

type rowScanner interface {
	Scan(...any) error
}

func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(ctx)
}
