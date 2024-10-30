package storage

type DocumentSummary struct {
	id string
}

func (ds *DocumentSummary) Id() string {
	return ds.id
}

type Storage interface {
}
