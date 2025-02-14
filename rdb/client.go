// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of MQUERY.
//
//  MQUERY is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  MQUERY is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with MQUERY.  If not, see <https://www.gnu.org/licenses/>.

package rdb

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/czcorpus/mquery-sru/result"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	MsgNewQuery                = "newQuery"
	MsgNewResult               = "newResult"
	DefaultQueueKey            = "mqueryQueue"
	DefaultResultChannelPrefix = "mqueryResults"
	DefaultQueryChannel        = "mqueryQueries"
	DefaultResultExpiration    = 10 * time.Minute
	DefaultQueryAnswerTimeout  = 60 * time.Second
)

var (
	ErrorEmptyQueue = errors.New("no queries in the queue")
)

type Query struct {
	Channel string        `json:"channel"`
	Func    string        `json:"func"`
	Args    ConcQueryArgs `json:"args"`
}

type ConcQueryArgs struct {
	CorpusPath        string   `json:"corpusPath"`
	Query             string   `json:"query"`
	Attrs             []string `json:"attrs"`
	MaxItems          int      `json:"maxItems"`
	StartLine         int      `json:"startLine"`
	MaxContext        int      `json:"maxContext"`
	ViewContextStruct string   `json:"viewContextStruct"`
}

func (q Query) ToJSON() (string, error) {
	ans, err := json.Marshal(q)
	if err != nil {
		return "", err
	}
	return string(ans), nil
}

func DecodeQuery(q string) (Query, error) {
	var ans Query
	var buff bytes.Buffer
	buff.WriteString(q)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&ans)
	return ans, err
}

type TimeoutError struct {
	Msg string
}

func (err TimeoutError) Error() string {
	return err.Msg
}

// --------------------

type TransmittedError struct {
	Message string
	Type    string
}

func (err *TransmittedError) Error() string {
	return fmt.Sprintf("TransmittedError(%s: %s)", err.Type, err.Message)
}

//

// Adapter provides functions for query producers and consumers
// using Redis database. It leverages Redis' PUBSUB functionality
// to notify about incoming data.
type Adapter struct {
	ctx                 context.Context
	redis               *redis.Client
	conf                *Conf
	channelQuery        string
	channelResultPrefix string
	queryAnswerTimeout  time.Duration
}

func (a *Adapter) TestConnection(totalTimeout time.Duration, timeoutPerTry time.Duration) error {
	ctx, cancelFn := context.WithTimeout(a.ctx, totalTimeout)
	defer cancelFn()
	tick := time.NewTicker(2 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed to connect to the Redis server at %s", a.conf.ServerInfo())
		case <-tick.C:
			log.Info().
				Str("server", a.conf.ServerInfo()).
				Msg("waiting for Redis server...")
			ctx2, cancelFn2 := context.WithTimeout(ctx, timeoutPerTry)
			_, err := a.redis.Ping(ctx2).Result()
			cancelFn2()
			if err != nil {
				log.Error().Err(err).Msg("...failed to get response from Redis server")

			} else {
				return nil
			}
		}
	}
}

// SomeoneListens tests if there is a listener for a channel
// specified in the provided `query`. If false, then there
// is nobody interested in the query anymore.
func (a *Adapter) SomeoneListens(query Query) (bool, error) {
	cmd := a.redis.PubSubNumSub(a.ctx, query.Channel)
	if cmd.Err() != nil {
		return false, fmt.Errorf("failed to check channel listeners: %w", cmd.Err())
	}
	return cmd.Val()[query.Channel] > 0, nil
}

// PublishQuery publishes a new query and returns a channel
// by which a respective result will be returned. In case the
// process fails during the calculation, a corresponding error
// is added to the ConcResult value.
// If the PublishQuery method itself returns an error, it means,
// that the publishing itself failed and the client won't obtain
// any information about the calculation (in which case it relies
// on timeout)
func (a *Adapter) PublishQuery(query Query) (<-chan result.ConcResult, error) {
	query.Channel = fmt.Sprintf("%s:%s", a.channelResultPrefix, uuid.New().String())
	log.Debug().
		Str("channel", query.Channel).
		Str("func", query.Func).
		Any("args", query.Args).
		Msg("publishing query")

	var msg bytes.Buffer
	enc := gob.NewEncoder(&msg)
	err := enc.Encode(query)
	if err != nil {
		return nil, fmt.Errorf("failed to publish query: %w", err)
	}

	ctx2, cancel := context.WithTimeout(a.ctx, a.queryAnswerTimeout)
	defer cancel()
	sub := a.redis.Subscribe(ctx2, query.Channel)
	if err := a.redis.LPush(ctx2, DefaultQueueKey, msg.String()).Err(); err != nil {
		return nil, err
	}
	ansChan := make(chan result.ConcResult)

	// now we wait for response and send result via `ans`
	go func() {
		defer func() {
			sub.Close()
			close(ansChan)
		}()

		ctx3, cancel := context.WithTimeout(a.ctx, a.queryAnswerTimeout)
		defer cancel()
		var ans result.ConcResult

		for {
			select {
			case item, ok := <-sub.Channel():
				log.Debug().
					Str("channel", query.Channel).
					Bool("closedChannel", !ok).
					Msg("received result")
				cmd := a.redis.Get(ctx3, item.Payload)
				if cmd.Err() != nil {
					ans.Error = cmd.Err()

				} else {
					var buf bytes.Buffer
					buf.WriteString(cmd.Val())
					dec := gob.NewDecoder(&buf)
					err := dec.Decode(&ans)
					if err != nil {
						ans.Error = err
					}
					log.Debug().
						Str("channel", query.Channel).
						Int("concSize", ans.ConcSize).
						Str("query", ans.Query).
						Msg("decoded result")
				}
				ansChan <- ans
				return
			case <-ctx3.Done():
				ans.Error = fmt.Errorf("waiting for worker response timeout")
				ansChan <- ans
			case <-a.ctx.Done():
				log.Warn().Msg("publishing query interrupted due to cancellation")
				return
			}
		}

	}()
	return ansChan, a.redis.Publish(ctx2, a.channelQuery, MsgNewQuery).Err()
}

// DequeueQuery looks for a query queued for processing.
// In case nothing is found, ErrorEmptyQueue is returned
// as an error.
func (a *Adapter) DequeueQuery() (Query, error) {
	cmd := a.redis.RPop(a.ctx, DefaultQueueKey)

	if cmd.Val() == "" {
		return Query{}, ErrorEmptyQueue
	}
	if cmd.Err() != nil {
		return Query{}, fmt.Errorf("failed to dequeue query: %w", cmd.Err())
	}
	q, err := DecodeQuery(cmd.Val())
	if err != nil {
		return Query{}, fmt.Errorf("failed to deserialize query: %w", err)
	}
	return q, nil
}

// PublishResult sends notification via Redis PUBSUB mechanism
// and also stores the result so a notified listener can retrieve
// it.
func (a *Adapter) PublishResult(channelName string, value *result.ConcResult) error {
	log.Debug().
		Str("channel", channelName).
		Str("resultType", "concordance").
		Msg("publishing result")

	if value.Error != nil {
		value.Error = &TransmittedError{
			Message: value.Error.Error(), Type: fmt.Sprintf("%T", value.Error)}
	}

	var msg bytes.Buffer
	enc := gob.NewEncoder(&msg)
	err := enc.Encode(value)
	if err != nil {
		return fmt.Errorf("failed to serialize (GOB) result: %w", err)
	}
	a.redis.Set(a.ctx, channelName, msg.String(), DefaultResultExpiration)
	return a.redis.Publish(a.ctx, channelName, channelName).Err()
}

// Subscribe subscribes to query queue.
func (a *Adapter) Subscribe() <-chan *redis.Message {
	sub := a.redis.Subscribe(a.ctx, a.channelQuery)
	return sub.Channel()
}

// NewAdapter is a recommended factory function
// for creating new `Adapter` instances
func NewAdapter(ctx context.Context, conf *Conf) *Adapter {
	chRes := conf.ChannelResultPrefix
	chQuery := conf.ChannelQuery
	if chRes == "" {
		chRes = DefaultResultChannelPrefix
		log.Warn().
			Str("channel", chRes).
			Msg("Redis channel for results not specified, using default")
	}
	if chQuery == "" {
		chQuery := DefaultQueryChannel
		log.Warn().
			Str("channel", chQuery).
			Msg("Redis channel for queries not specified, using default")
	}
	queryAnswerTimeout := time.Duration(conf.QueryAnswerTimeoutSecs) * time.Second
	if queryAnswerTimeout == 0 {
		queryAnswerTimeout = DefaultQueryAnswerTimeout
		log.Warn().
			Float64("value", queryAnswerTimeout.Seconds()).
			Msg("queryAnswerTimeoutSecs not specified for Redis adapter, using default")
	}
	ans := &Adapter{
		conf: conf,
		redis: redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", conf.Host, conf.Port),
			Password: conf.Password,
			DB:       conf.DB,
		}),
		ctx:                 ctx,
		channelQuery:        chQuery,
		channelResultPrefix: chRes,
		queryAnswerTimeout:  queryAnswerTimeout,
	}
	return ans
}
