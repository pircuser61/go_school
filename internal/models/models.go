package models

import "time"

type Material struct {
	UUID     string
	Type     string    // Тип материала, значения: статья, видеоролик, презентация
	Status   int       // Статус публикации, значения: 2 архивный, 1 активный
	Name     string    // Название - краткое название материала
	Content  string    // Содержание - текстовое описание материала
	DtCreate time.Time //Дата создания - проставляется автоматически
	DtUpdate time.Time //Дата изменения - проставляется автоматически
}

type MaterialListFilter struct {
	Type         string
	DtCreateFrom time.Time
	DtCreateTo   time.Time
	Offset       int
	Limit        int
}
