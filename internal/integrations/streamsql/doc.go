// Package streamsql provides a SQL engine for defining real-time features
// using standard SQL over event streams.
//
// It supports a streaming SQL subset including SELECT, FROM, WHERE, GROUP BY,
// HAVING, WINDOW (TUMBLE/SLIDE), ORDER BY, LIMIT, and EMIT clauses. The engine
// processes records pushed into named streams and evaluates registered queries
// continuously or on demand.
//
// Usage:
//
//	engine := streamsql.NewEngine(streamsql.DefaultEngineConfig())
//	engine.CreateStream("clicks", map[string]string{"user_id": "string", "count": "int"})
//	engine.Push("clicks", &streamsql.Record{
//	    Fields:    map[string]interface{}{"user_id": "u1", "count": 1},
//	    Timestamp: time.Now(),
//	})
//	result, err := engine.ExecuteQuery(ctx, "SELECT user_id, SUM(count) FROM clicks GROUP BY user_id")
package streamsql
