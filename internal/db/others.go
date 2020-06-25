package db

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"regexp"
	"strings"
)

func RenameFuncs(d *dbconn.PGConnection) error  {
	q := `select id, content from pipeliner.versions`

	rows, err := d.Pool.Query(context.Background(), q)
	if err != nil {
		return err
	}
	for rows.Next() {
		id := uuid.UUID{}
		cont := ""
		err = rows.Scan(&id, &cont)
		r, err := regexp.Compile(`2g-[a-z-]*`)
		if err != nil {
			return nil
		}
		matches := r.FindAll([]byte(cont), -1)
		newCont := cont
		for _, s := range matches {
			oldS := string(s)
			newS := strings.ReplaceAll(oldS, "2g-", "") + "-2g"
			fmt.Println(oldS, newS)
			newCont = strings.ReplaceAll(newCont, oldS, newS)

		}
		ins := `update pipeliner.versions set content=$1 where id=$2`
		_, err = d.Pool.Exec(context.Background(), ins, newCont, id)
		if err != nil {
			return err
		}
	}
	return nil
}