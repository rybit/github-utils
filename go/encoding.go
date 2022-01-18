package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

type encoder func(obj interface{}) error

func buildJSONEncoder(out io.WriteCloser) encoder {
	writer := json.NewEncoder(out)
	return func(obj interface{}) error {
		return writer.Encode(&obj)
	}
}

func buildCSVEncoder(out io.WriteCloser) encoder {
	writer := csv.NewWriter(out)
	var once sync.Once

	return func(obj interface{}) error {
		encObj, ok := obj.(csvWritable)
		if !ok {
			return errors.New("object doesn't implement csvWritable")
		}

		var headers []string
		var entries []string
		for _, f := range encObj.Fields() {
			headers = append(headers, f.header)

			entries = append(entries, fmt.Sprintf("%v", f.value))
		}
		once.Do(func() {
			panicOnErr(writer.Write(headers))
		})

		panicOnErr(writer.Write(entries))

		writer.Flush()
		return writer.Error()
	}
}

type csvField struct {
	header string
	value  interface{}
}

type csvWritable interface {
	Fields() []csvField
}
