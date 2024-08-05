package models

import (
	"time"

	"github.com/google/uuid"
)

type Material struct {
	UUID     uuid.UUID
	Type     string     // Тип материала, значения: статья, видеоролик, презентация
	Status   string     // Статус публикации, значения: 2 архивный, 1 активный
	Title    string     // Название - краткое название материала
	Content  string     // Содержание - текстовое описание материала
	DtCreate time.Time  //Дата создания - проставляется автоматически
	DtUpdate *time.Time //Дата изменения - проставляется автоматически, NULLABLE
}

type MaterialListItem struct {
	UUID     uuid.UUID
	Type     string
	Title    string
	DtCreate time.Time
	DtUpdate *time.Time
}

type MaterialListFilter struct {
	Type         string     `schema:"type"`
	DtCreateFrom *time.Time `schema:"cr_from"`
	DtCreateTo   *time.Time `schema:"cr_to"`
	Offset       uint64     `schema:"offset"`
	Limit        uint64     `schema:"limit"`
}
