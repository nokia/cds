package sdk

import "time"

const (
	EntityTypeWorkerModelTemplate = "WorkerModelTemplate"
	EntityTypeWorkerModel         = "WorkerModel"
)

type Entity struct {
	ID                  string    `json:"id" db:"id"`
	ProjectKey          string    `json:"project_key" db:"project_key"`
	ProjectRepositoryID string    `json:"project_repository_id" db:"project_repository_id"`
	Type                string    `json:"type" db:"type"`
	Name                string    `json:"name" db:"name"`
	Branch              string    `json:"branch" db:"branch"`
	Commit              string    `json:"commit" db:"commit"`
	LastUpdate          time.Time `json:"last_update" db:"last_update"`
	Data                string    `json:"data" db:"data"`
}
