package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	pgxpool "github.com/jackc/pgx/v5/pgxpool"

	config "github.com/pircuser61/go_school/config"
	models "github.com/pircuser61/go_school/internal/models"
	storage "github.com/pircuser61/go_school/internal/storage"
)

type PostgresStore struct {
	l       *slog.Logger
	Timeout time.Duration
	pool    *pgxpool.Pool
}

func New(ctx context.Context, logger *slog.Logger) (storage.MateralStore, error) {
	logger.Info("pg:Подключение к БД...")
	pool, err := pgxpool.New(ctx, config.GetConnectionString())
	if err != nil {
		return nil, err
	}
	logger.Info("pg:Миграции...")
	err = MakeMigrations(pool)
	if err != nil {
		return nil, err
	}
	store := PostgresStore{l: logger, pool: pool}
	return store, nil
}

func (i PostgresStore) Close() {
	i.l.Info("pg:Соедиение с БД закрыто ")
}

func (i PostgresStore) foreignKeys(ctx context.Context, m models.Material) (int, int, error) {
	const queryGetForeign = `SELECT t.id, s.id 
		FROM material_type t, material_status s
		WHERE t.name = $1 
		AND s.name = $2;`
	i.l.Debug("pg:запрос справочников", slog.String("name", m.Title))
	var mType, mStatus int
	row := i.pool.QueryRow(ctx, queryGetForeign, m.Type, m.Status)
	err := row.Scan(&mType, &mStatus)
	if err != nil {
		i.l.Error("pg:Material Create", slog.String("msg", err.Error()))

	}
	return mType, mStatus, err
}

func (i PostgresStore) MaterialCreate(ctx context.Context, m models.Material) (uuid.UUID, error) {

	const queryAdd = `INSERT INTO material 
		(uuid, title, type, status, content, dt_create)
		VALUES ($1, $2, $3, $4, $5, now());`
	mType, mStatus, err := i.foreignKeys(ctx, m)
	if err != nil {
		return uuid.Nil, err
	}
	m.UUID = uuid.New()
	i.l.Debug("pg:Добавление материала", slog.String("name", m.Title))
	_, err = i.pool.Exec(ctx, queryAdd,
		m.UUID, m.Title, mType, mStatus, m.Content)

	if err != nil {
		i.l.Error("pg:Material Create", slog.String("msg", err.Error()))
		return uuid.Nil, err
	}
	return m.UUID, err
}

func (i PostgresStore) MaterialGet(ctx context.Context, id uuid.UUID) (models.Material, error) {
	i.l.Debug("pg:Получение материала", slog.Any("id", id))
	const queryGet = `SELECT uuid, 
			material_status.name as status, material_type.name as name, 
			title, content, dt_create, dt_update
		FROM material
		LEFT OUTER JOIN material_type ON material_type.id = material.type
		LEFT OUTER JOIN material_status ON material_status.id = material.status
		WHERE uuid = $1;`

	var m models.Material
	row := i.pool.QueryRow(ctx, queryGet, id)

	err := row.Scan(&m.UUID, &m.Status, &m.Type, &m.Title, &m.Content, &m.DtCreate, &m.DtUpdate)
	if err != nil {
		i.l.Error("pg:Material Get", slog.String("msg", err.Error()))
	}
	return m, err
}

func (i PostgresStore) MaterialUpdate(ctx context.Context, m models.Material) error {
	const queryAdd = `UPDATE material 
		SET title =$2, status=$4, content = $5, dt_update = now()
		WHERE uuid = $1	AND type=$3;`
	mType, mStatus, err := i.foreignKeys(ctx, m)
	if err != nil {
		return err
	}
	i.l.Debug("pg:Обновление материала", slog.Any("id", m.UUID))
	commandTag, err := i.pool.Exec(ctx, queryAdd, m.UUID, m.Title, mType, mStatus,
		m.Content)

	if err != nil {
		i.l.Error("pg:Material Update", slog.String("msg", err.Error()))
		return err
	}
	if commandTag.RowsAffected() == 1 {
		return nil
	}
	oldM, err := i.MaterialGet(ctx, m.UUID)
	if err == nil {
		if oldM.Type != m.Type {
			return fmt.Errorf("нельзя изменять тип материала %s %s", oldM.Type, m.Type)
		}
		return fmt.Errorf("запись найдена в базе, но не удалось обновить")
	}
	return err
}

func (i PostgresStore) MaterialDelete(ctx context.Context, id uuid.UUID) error {
	i.l.Debug("pg:Удаление материала", slog.Any("id", id))
	const queryDelete = `DELETE FROM material WHERE uuid = $1;`

	commandTag, err := i.pool.Exec(ctx, queryDelete, id)
	if err != nil {
		i.l.Error("pg:Material: Delete", slog.String("msg", err.Error()))
		return err
	}
	if commandTag.RowsAffected() != 1 {
		return fmt.Errorf("не найдена запись с ID %s", id)
	}
	return nil
}

func (i PostgresStore) Materials(ctx context.Context, filter models.MaterialListFilter) ([]models.MaterialListItem, error) {
	/*
		LEFT OUTER JOIN material_type ON material_type.id = material.type
		LEFT OUTER JOIN material_status ON material_status.id = material.status
	*/
	i.l.Debug("pg:Запрос списка", slog.Any("Фильтр", filter))

	if filter.Limit <= 0 || filter.Limit > 20 {
		filter.Limit = 20
	}
	sql := sq.Select("UUID, material_type.name as type, title, dt_create, dt_update").From("material").
		Join("material_type on material_type.id =  type").
		Limit(filter.Limit).PlaceholderFormat(sq.Dollar)

	if filter.Offset > 0 {
		sql = sql.Offset(filter.Offset)
	}
	/*
		if filter.DtCreateFrom != nil && filter.DtCreateTo != nil {
			sql = sql.Where(sq.GtOrEq{"DT_CREATE": filter.DtCreateFrom}, sq.LtOrEq{"DT_CREATE": filter.DtCreateTo})
		}
	*/
	if filter.DtCreateFrom != nil {
		sql = sql.Where(sq.GtOrEq{"DT_CREATE": filter.DtCreateFrom})
	}
	if filter.DtCreateTo != nil {
		sql = sql.Where(sq.LtOrEq{"DT_CREATE": filter.DtCreateTo})
	}

	queryList, args, err := sql.ToSql()
	if err != nil {
		return nil, err
	}
	i.l.Debug("pg:Запрос списка", slog.Any("Query", queryList), slog.Any("Args", args))
	rows, err := i.pool.Query(ctx, queryList, args...)
	if err != nil {
		i.l.Error("pg:Materials Query", slog.String("msg", err.Error()))
		return nil, err
	}

	result, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.MaterialListItem])
	if err != nil {
		i.l.Error("pg:Materials Parse", slog.String("msg", err.Error()))
		return nil, err
	}
	return result, nil
}
