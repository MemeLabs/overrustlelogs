package common

import (
	"bytes"
	"context"
	"log"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/option"
)

// BigQueryWriterConfig ...
type BigQueryWriterConfig struct {
	ProjectID          string `json:"projectID"`
	DatasetID          string `json:"datasetID"`
	TableID            string `json:"tableID"`
	ServiceAccountJSON string `json:"serviceAccountJSON"`
}

// BigQueryWriter ...
type BigQueryWriter struct {
	client *bigquery.Client
	config BigQueryWriterConfig
}

// NewBigQueryWriter ...
func NewBigQueryWriter(config BigQueryWriterConfig) (*BigQueryWriter, error) {
	client, err := bigquery.NewClient(
		context.Background(),
		config.ProjectID,
		option.WithServiceAccountFile(config.ServiceAccountJSON),
	)
	if err != nil {
		return nil, err
	}

	return &BigQueryWriter{client, config}, nil
}

// Write ...
func (w *BigQueryWriter) Write(b []byte) (int, error) {
	source := bigquery.NewReaderSource(bytes.NewReader(b))
	source.AllowJaggedRows = true
	source.SourceFormat = bigquery.Avro

	loader := w.client.Dataset(w.config.DatasetID).Table(w.config.TableID).LoaderFrom(source)
	loader.CreateDisposition = bigquery.CreateIfNeeded
	loader.WriteDisposition = bigquery.WriteAppend

	job, err := loader.Run(context.Background())
	if err != nil {
		return 0, err
	}
	status, err := job.Wait(context.Background())
	if err != nil {
		return 0, err
	}
	if err := status.Err(); err != nil {
		return 0, err
	}

	stats := status.Statistics.Details.(*bigquery.LoadStatistics)
	log.Printf(
		"finished loading data into bigquery (TotalBytesProcessed: %d, InputFileBytes: %d, OutputBytes: %d, OutputRows: %d)",
		status.Statistics.TotalBytesProcessed,
		stats.InputFileBytes,
		stats.OutputBytes,
		stats.OutputRows,
	)
	return len(b), nil
}
