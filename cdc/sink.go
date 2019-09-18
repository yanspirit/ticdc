// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package cdc

import (
	"context"
	"database/sql"
	"fmt"
	"io"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/parser/model"
)

// Sink is an abstraction for anything that a changefeed may emit into.
type Sink interface {
	Emit(ctx context.Context, txn Txn) error
	EmitResolvedTimestamp(
		ctx context.Context,
		encoder Encoder,
		resolved uint64,
	) error
	// TODO: Add GetLastSuccessTs() uint64
	// Flush blocks until every message enqueued by EmitRow and
	// EmitResolvedTimestamp has been acknowledged by the sink.
	Flush(ctx context.Context) error
	// Close does not guarantee delivery of outstanding messages.
	Close() error
}

// TableInfoGetter is used to get table info by table id of TiDB
type TableInfoGetter interface {
	TableByID(id int64) (info *model.TableInfo, ok bool)
	GetTableIDByName(schema, table string) (int64, bool)
}

func getSink(
	sinkURI string,
	infoGetter TableInfoGetter,
	opts map[string]string,
) (Sink, error) {
	// TODO
	db, err := sql.Open("mysql", sinkURI)
	if err != nil {
		return nil, err
	}
	cachedInspector := newCachedInspector(db)
	sink := mysqlSink{
		db:           db,
		infoGetter:   infoGetter,
		tblInspector: cachedInspector,
	}
	return &sink, nil
}

type writerSink struct {
	io.Writer
}

var _ Sink = &writerSink{}

func (s *writerSink) Emit(ctx context.Context, txn Txn) error {
	fmt.Fprintf(s, "commit ts: %d", txn.Ts)
	return nil
}

func (s *writerSink) EmitResolvedTimestamp(ctx context.Context, encoder Encoder, resolved uint64) error {
	fmt.Fprintf(s, "resolved: %d", resolved)
	return nil
}

func (s *writerSink) Flush(ctx context.Context) error {
	return nil
}

func (s *writerSink) Close() error {
	return nil
}
