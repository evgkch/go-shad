//go:build !solution

package dao

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
)

type dao struct {
	pool *pgxpool.Pool
}

func (d *dao) Create(ctx context.Context, u *User) (UserID, error) {
	var id UserID

	err := d.pool.QueryRow(
		ctx,
		`INSERT INTO users (name) VALUES ($1) RETURNING id`,
		u.Name,
	).Scan(&id)

	if err != nil {
		return 0, err
	}

	return UserID(id), nil
}

func (d *dao) Update(ctx context.Context, u *User) error {
	tag, err := d.pool.Exec(
		ctx,
		`UPDATE users SET name = $1 WHERE id = $2`,
		u.Name,
		u.ID,
	)

	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (d *dao) Delete(ctx context.Context, id UserID) error {
	tag, err := d.pool.Exec(
		ctx,
		`DELETE FROM users WHERE id = $1`,
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

func (d *dao) Lookup(ctx context.Context, id UserID) (User, error) {
	var user User

	err := d.pool.QueryRow(
		ctx,
		`SELECT * from users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Name)

	if err != nil {
		return User{}, err
	}

	return user, err
}

func (d *dao) List(ctx context.Context) ([]User, error) {
	rows, err := d.pool.Query(ctx, `SELECT id, name FROM users`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name); err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if rows.Err() != nil {
		return nil, err
	}

	return users, nil
}

func (d *dao) Close() error {
	d.pool.Close()
	return nil
}

func CreateDao(ctx context.Context, dsn string) (Dao, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users(
			id   BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL
		)
	`)

	if err != nil {
		pool.Close()
		return nil, err
	}

	return &dao{
		pool: pool,
	}, nil
}
