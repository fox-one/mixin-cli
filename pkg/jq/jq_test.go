package jq

import (
	"encoding/json"
	"testing"

	"github.com/fox-one/pando/cmd/pando-cli/internal/column"
	"github.com/stretchr/testify/require"
)

func TestParseObject(t *testing.T) {
	m := map[string]interface{}{
		"foo":  "bar",
		"age":  10,
		"name": "lucy",
	}

	b, _ := json.Marshal(m)
	results, err := ParseObject(b, "age", "foo", "name")
	require.Nil(t, err)
	t.Log(column.Print(results))
}

func TestParseObjects(t *testing.T) {
	m := map[string]interface{}{
		"foo":     "bar",
		"age":     10,
		"name":    "lucy",
		"user_id": 100,
	}

	objects := []interface{}{m, m, m}
	b, _ := json.Marshal(objects)

	results, err := ParseObjects(b, "user_id", "age", "foo", "name")
	require.Nil(t, err)
	t.Log(column.Print(results))
}
