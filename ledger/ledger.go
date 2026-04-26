//go:build !solution

package ledger

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ledger struct {
	pool *pgxpool.Pool
}

func (l *ledger) CreateAccount(ctx context.Context, id ID) error {
	_, err := l.pool.Exec(
		ctx,
		`INSERT INTO users (id, balance) values ($1, 0)`,
		id,
	)
	if err != nil {
		return err
	}

	return nil
}

func (l *ledger) GetBalance(ctx context.Context, id ID) (Money, error) {
	var balance Money

	err := l.pool.QueryRow(
		ctx,
		`SELECT balance FROM users WHERE id = $1`,
		id,
	).Scan(&balance)

	if err != nil {
		return 0, err
	}

	return balance, nil
}

func (l *ledger) Deposit(ctx context.Context, id ID, amount Money) error {
	if amount < 0 {
		return ErrNegativeAmount
	}

	tag, err := l.pool.Exec(
		ctx,
		`UPDATE users SET balance = balance + $1 WHERE id = $2`,
		amount,
		id,
	)

	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (l *ledger) Withdraw(ctx context.Context, id ID, amount Money) error {
	if amount < 0 {
		return ErrNegativeAmount
	}

	tx, err := l.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var balance Money
	if err := tx.QueryRow(
		ctx,
		`SELECT balance FROM users WHERE id = $1 FOR UPDATE`,
		id,
	).Scan(&balance); err != nil {
		return err
	}

	if balance < amount {
		return ErrNoMoney
	}

	if _, err := tx.Exec(
		ctx,
		`UPDATE users SET balance = balance - $1 WHERE id = $2`,
		amount,
		id,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (l *ledger) Transfer(ctx context.Context, from, to ID, amount Money) error {
	if amount < 0 {
		return ErrNegativeAmount
	}

	tx, err := l.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	firstID, secondID := from, to
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	var firstBalance Money
	if err := tx.QueryRow(
		ctx,
		`SELECT balance FROM users WHERE id = $1 FOR UPDATE`,
		firstID,
	).Scan(&firstBalance); err != nil {
		return err
	}

	var secondBalance Money
	if secondID != firstID {
		if err := tx.QueryRow(
			ctx,
			`SELECT balance FROM users WHERE id = $1 FOR UPDATE`,
			secondID,
		).Scan(&secondBalance); err != nil {
			return err
		}
	} else {
		secondBalance = firstBalance
	}

	fromBalance := firstBalance
	if from != firstID {
		fromBalance = secondBalance
	}
	if fromBalance < amount {
		return ErrNoMoney
	}

	if from == to {
		return tx.Commit(ctx)
	}

	tag, err := tx.Exec(
		ctx,
		`UPDATE users SET balance = balance - $1 WHERE id = $2`,
		amount,
		from,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	tag, err = tx.Exec(
		ctx,
		`UPDATE users SET balance = balance + $1 WHERE id = $2`,
		amount,
		to,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return tx.Commit(ctx)
}

func (l *ledger) Close() error {
	l.pool.Close()
	return nil
}

func New(ctx context.Context, dsn string) (Ledger, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	_, err = pool.Exec(
		ctx,
		`CREATE TABLE IF NOT EXISTS users(
			id 	    TEXT PRIMARY KEY,
			balance BIGINT DEFAULT 0 CHECK (balance >= 0)
		)`,
	)

	if err != nil {
		pool.Close()
		return nil, err
	}

	return &ledger{pool: pool}, nil
}
