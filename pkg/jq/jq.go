package jq

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/itchyny/gojq"
	"github.com/spf13/cast"
)

func Parse(v interface{}, query string) ([]string, error) {
	q, err := gojq.Parse(query)
	if err != nil {
		return nil, err
	}

	var outputs []string
	iter := q.Run(v)

	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, err
		}

		outputs = append(outputs, cast.ToString(v))
	}

	return outputs, nil
}

func formatFields(fields []string, actions ...string) []string {
	formatted := make([]string, len(fields))
	for i, field := range fields {
		field = "." + field
		if len(actions) > 0 {
			field = fmt.Sprintf("(%s|%s)", field, strings.Join(actions, ","))
		}

		formatted[i] = field
	}

	return formatted
}

func buildObjectsQuery(fields ...string) string {
	return ".[] | " + strings.Join(formatFields(fields, "tostring"), ` + "," + `)
}

func ParseObjects(data []byte, fields ...string) ([]string, error) {
	query := buildObjectsQuery(fields...)

	var objects []interface{}
	if err := json.Unmarshal(data, &objects); err != nil {
		return nil, err
	}

	results, err := Parse(objects, query)
	if err != nil {
		return nil, err
	}

	headers := make([]string, len(fields))
	for i, field := range fields {
		headers[i] = strcase.ToCamel(field)
	}

	return append([]string{strings.Join(headers, ",")}, results...), nil
}

func buildObjectQuery(fields ...string) string {
	return strings.Join(formatFields(fields), `,`)
}

func ParseObject(data []byte, fields ...string) ([]string, error) {
	query := buildObjectQuery(fields...)

	object := make(map[string]interface{})
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}

	results, err := Parse(object, query)
	if err != nil {
		return nil, err
	}

	for i, result := range results {
		results[i] = strcase.ToCamel(fields[i]) + "," + result
	}

	return results, nil
}
