package n8n

import (
	"context"
	"time"
)

func SetSleepForTest(c *Client, f func(context.Context, time.Duration) error) { c.sleep = f }

func SetClockForTest(c *Client, now func() time.Time) { c.cache.now = now }
