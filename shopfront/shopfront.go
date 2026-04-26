//go:build !solution

package shopfront

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

type counters struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) Counters {
	return &counters{rdb: rdb}
}

func (c *counters) GetItems(ctx context.Context, ids []ItemID, userID UserID) ([]Item, error) {
	items := make([]Item, len(ids))
	countCmds := make([]*redis.StringCmd, len(ids))
	viewedCmds := make([]*redis.BoolCmd, len(ids))

	_, err := c.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		for i, id := range ids {
			countCmds[i] = p.Get(ctx, countKey(id))
			viewedCmds[i] = p.SIsMember(ctx, viewedKey(id), int64(userID))
		}

		return nil
	})
	if err != nil && err != redis.Nil {
		return nil, err
	}

	for i := range ids {
		viewCount, err := countCmds[i].Int64()
		if err != nil && err != redis.Nil {
			return nil, err
		}
		viewed, err := viewedCmds[i].Result()
		if err != nil {
			return nil, err
		}

		items[i] = Item{
			ViewCount: int(viewCount),
			Viewed:    viewed,
		}
	}

	return items, nil
}

func (c *counters) RecordView(ctx context.Context, id ItemID, userID UserID) error {
	_, err := c.rdb.Pipelined(ctx, func(p redis.Pipeliner) error {
		p.Incr(ctx, countKey(id))
		p.SAdd(ctx, viewedKey(id), int64(userID))
		return nil
	})

	return err
}

func countKey(id ItemID) string {
	return fmt.Sprintf("shopfront:item:%d:count", id)
}

func viewedKey(id ItemID) string {
	return fmt.Sprintf("shopfront:item:%d:viewed", id)
}
