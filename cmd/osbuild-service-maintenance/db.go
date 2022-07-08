package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/sirupsen/logrus"
)

const (
	// Maintenance queries
	sqlDeleteJob = `
                DELETE FROM jobs
                WHERE id IN (
                    SELECT id FROM jobs
                    WHERE expires_at < NOW()
                    ORDER BY expires_at
                    LIMIT 1000
                )`
	sqlVacuumAnalyze = `
                VACUUM ANALYZE`
	sqlVacuumStats = `
                SELECT relname, pg_size_pretty(pg_total_relation_size(relid)),
                    n_tup_ins, n_tup_upd, n_tup_del, n_live_tup, n_dead_tup,
                    vacuum_count, autovacuum_count, analyze_count, autoanalyze_count,
                    last_vacuum, last_autovacuum, last_analyze, last_autoanalyze
                 FROM pg_stat_user_tables`
)

type db struct {
	Conn *pgx.Conn
}

func newDB(dbURL string) (db, error) {
	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		return db{}, err
	}

	return db{
		conn,
	}, nil
}

func (d *db) Close() {
	d.Conn.Close(context.Background())
}

func (d *db) DeleteJob() (int64, error) {
	tag, err := d.Conn.Exec(context.Background(), sqlDeleteJob)
	if err != nil {
		return tag.RowsAffected(), fmt.Errorf("Error deleting results from jobs: %v", err)
	}
	return tag.RowsAffected(), nil
}

func (d *db) VacuumAnalyze() error {
	_, err := d.Conn.Exec(context.Background(), sqlVacuumAnalyze)
	if err != nil {
		return fmt.Errorf("Error running VACUUM ANALYZE: %v", err)
	}
	return nil
}

func (d *db) LogVacuumStats() error {
	rows, err := d.Conn.Query(context.Background(), sqlVacuumStats)
	if err != nil {
		return fmt.Errorf("Error querying vacuum stats: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var relName, relSize string
		var ins, upd, del, live, dead, vc, avc, ac, aac int64
		var lvc, lavc, lan, laan *time.Time

		err = rows.Scan(&relName, &relSize, &ins, &upd, &del, &live, &dead,
			&vc, &avc, &ac, &aac,
			&lvc, &lavc, &lan, &laan)
		if err != nil {
			return err
		}

		logrus.Infof("Stats for table %s", relName)
		logrus.Infof("  Total table size: %s", relSize)
		logrus.Info("  Tuples:")
		logrus.Infof("    Inserted: %d", ins)
		logrus.Infof("    Updated: %d", upd)
		logrus.Infof("    Deleted: %d", del)
		logrus.Infof("    Live: %d", live)
		logrus.Infof("    Dead: %d", dead)
		logrus.Info("  Vacuum stats:")
		logrus.Infof("    Vacuum count: %d", vc)
		logrus.Infof("    AutoVacuum count: %d", avc)
		logrus.Infof("    Last vacuum: %v", lvc)
		logrus.Infof("    Last autovacuum: %v", lavc)
		logrus.Info("  Analyze stats:")
		logrus.Infof("    Analyze count: %d", ac)
		logrus.Infof("    AutoAnalyze count: %d", aac)
		logrus.Infof("    Last analyze: %v", lan)
		logrus.Infof("    Last autoanalyze: %v", laan)
		logrus.Info("---")
	}
	if rows.Err() != nil {
		return rows.Err()
	}
	return nil

}

func DBCleanup(dbURL string, dryRun bool, cutoff time.Time) error {
	db, err := newDB(dbURL)
	if err != nil {
		return err
	}

	err = db.LogVacuumStats()
	if err != nil {
		logrus.Errorf("Error running vacuum stats: %v", err)
	}

	var rows int64

	for {
		rows, err = db.DeleteJob()

		if err != nil {
			logrus.Errorf("Error deleting results for jobs: %v, %d rows affected", rows, err)
			return err
		}

		if rows == 0 {
			break
		}

		logrus.Infof("Deleted results for %d", rows)
	}

	err = db.VacuumAnalyze()
	if err != nil {
		logrus.Errorf("Error running vacuum analyze: %v", err)
	}

	err = db.LogVacuumStats()
	if err != nil {
		logrus.Errorf("Error running vacuum stats: %v", err)
	}

	return nil
}
