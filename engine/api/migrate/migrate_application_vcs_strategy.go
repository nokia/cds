package migrate

import (
	"context"
	"database/sql"

	"github.com/go-gorp/gorp"
	"github.com/ovh/cds/engine/api/application"
	"github.com/ovh/cds/engine/api/workflow"
	"github.com/ovh/cds/engine/gorpmapper"
	"github.com/ovh/cds/sdk"
	"github.com/pkg/errors"
	"github.com/rockbears/log"
)

func ApplicationVCSStrategies(ctx context.Context, dbFunc func() *gorp.DbMap) error {
	appIDs, err := getApplicationsToFix(ctx, dbFunc())
	if err != nil {
		return err
	}

	for _, appID := range appIDs {
		tx, err := dbFunc().Begin()
		if err != nil {
			log.ErrorWithStackTrace(ctx, err)
			_ = tx.Rollback()
			continue
		}

		// lock the row
		if _, err := tx.Exec("select * from application where id = $1 for update skip locked", appID); err == sql.ErrNoRows {
			_ = tx.Rollback()
			continue
		}

		// load the full application
		app, err := application.LoadByIDWithClearVCSStrategyPassword(ctx, tx, appID)
		if err != nil {
			log.ErrorWithStackTrace(ctx, err)
			_ = tx.Rollback()
			continue
		}

		// Fin an old application from workflow runs
		oldApp, err := getApplicationWithVCSStrategyFromLatestRuns(ctx, tx, appID)
		if err != nil {
			log.ErrorWithStackTrace(ctx, err)
			_ = tx.Rollback()
			continue
		}

		// Copy the old repository strategy on the  current application
		app.RepositoryStrategy = oldApp.RepositoryStrategy

		// Save it
		if err := application.Update(ctx, tx, app); err != nil {
			log.ErrorWithStackTrace(ctx, err)
			_ = tx.Rollback()
			continue
		}

		log.Info(ctx, "Application %s/%s updated", app.ProjectKey, app.Name)

		if err := tx.Commit(); err != nil {
			log.ErrorWithStackTrace(ctx, err)
			_ = tx.Rollback()
			continue
		}

	}

	return nil
}

func getApplicationWithVCSStrategyFromLatestRuns(ctx context.Context, tx gorpmapper.SqlExecutorWithTx, appID int64) (*sdk.Application, error) {
	var selectWRIds []int64

	_, err := tx.Select(&selectWRIds, "SELECT distinct workflow_run_id FROM workflow_node_run WHERE application_id = $1 ORDER BY start DESC LIMIT 20", appID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Error(ctx, "No workflow run found for application %d", appID)
			return nil, nil
		}
		log.Error(ctx, "Cannot get workflow runs for application %d : %v", appID, err)
		return nil, err
	}

	for _, wrID := range selectWRIds {
		wr, err := workflow.LoadRunByID(ctx, tx, wrID, workflow.LoadRunOptions{})
		if err != nil {
			continue
		}
		app, has := wr.Workflow.Applications[appID]
		if !has {
			log.Error(ctx, "Workflow Run %d has no application %d", wr.ID, appID)
			continue
		}
		if app.RepositoryStrategy.ConnectionType == "" {
			log.Info(ctx, "Workflow Run %d has application %d with VCS strategy not set", wr.ID, appID)
			continue
		}
		for _, nrs := range wr.WorkflowNodeRuns {
			for _, nr := range nrs {
				if nr.ApplicationID == appID {
					secrets, err := workflow.LoadDecryptSecrets(ctx, tx, wr, &nr)
					if err != nil {
						log.Error(ctx, "Cannot load secrets for workflow run %d : %v", wr.ID, err)
						continue
					}
					vars := secrets.ToVariables()
					for _, v := range vars {
						if v.Name == "git.http.password" {
							app.RepositoryStrategy.Password = v.Value
						}
					}
					return &app, nil
				}
			}
		}

	}

	return nil, errors.Errorf("No workflow run found for application %d", appID)
}

func getApplicationsToFix(ctx context.Context, db *gorp.DbMap) ([]int64, error) {
	var selectAppIds []int64
	var appIds []int64
	_, err := db.Select(&appIds, "SELECT id FROM application")
	if err != nil {
		return nil, err
	}

	for _, appId := range appIds {
		app, err := application.LoadByIDWithClearVCSStrategyPassword(ctx, db, appId)
		if err != nil {
			return nil, err
		}

		if app.RepositoryFullname != "" && app.RepositoryStrategy.ConnectionType == "" {
			selectAppIds = append(selectAppIds, appId)
		}
	}

	return selectAppIds, nil
}
