package postgres

import "testing"

func TestParseQuery(t *testing.T) {
	tests := []struct {
		query     string
		operation string
		table     string
	}{
		{query: "select id from trips where id = $1", operation: "SELECT", table: "trips"},
		{query: "insert into trip_status_history (trip_id) values ($1)", operation: "INSERT", table: "trip_status_history"},
		{query: "update trips set status = $1 where id = $2", operation: "UPDATE", table: "trips"},
		{query: "delete from trip_status_history where trip_id = $1", operation: "DELETE", table: "trip_status_history"},
		{query: "begin", operation: "BEGIN", table: "unknown"},
		{query: "", operation: "QUERY", table: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.operation+" "+tt.table, func(t *testing.T) {
			operation, table := parseQuery(tt.query)
			if operation != tt.operation {
				t.Fatalf("operation %s", operation)
			}
			if table != tt.table {
				t.Fatalf("table %s", table)
			}
		})
	}
}
