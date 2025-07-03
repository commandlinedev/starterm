// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package wstore

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"time"

	"github.com/commandlinedev/starterm/pkg/filestore"
	"github.com/commandlinedev/starterm/pkg/panichandler"
	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/util/dbutil"
)

var ErrNotFound = fmt.Errorf("not found")

func starObjTableName(w starobj.StarObj) string {
	return "db_" + w.GetOType()
}

func tableNameFromOType(otype string) string {
	return "db_" + otype
}

func tableNameGen[T starobj.StarObj]() string {
	var zeroObj T
	return tableNameFromOType(zeroObj.GetOType())
}

func getOTypeGen[T starobj.StarObj]() string {
	var zeroObj T
	return zeroObj.GetOType()
}

func DBGetCount[T starobj.StarObj](ctx context.Context) (int, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) (int, error) {
		table := tableNameGen[T]()
		query := fmt.Sprintf("SELECT count(*) FROM %s", table)
		return tx.GetInt(query), nil
	})
}

// returns (num named workespaces, num total workspaces, error)
func DBGetWSCounts(ctx context.Context) (int, int, error) {
	var named, total int
	err := WithTx(ctx, func(tx *TxWrap) error {
		query := `SELECT count(*) FROM db_workspace WHERE COALESCE(json_extract(data, '$.name'), '') <> ''`
		named = tx.GetInt(query)
		query = `SELECT count(*) FROM db_workspace`
		total = tx.GetInt(query)
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return named, total, nil
}

var viewRe = regexp.MustCompile(`^[a-z0-9]{1,20}$`)

func DBGetBlockViewCounts(ctx context.Context) (map[string]int, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) (map[string]int, error) {
		query := `SELECT COALESCE(json_extract(data, '$.meta.view'), '') AS view FROM db_block`
		views := tx.SelectStrings(query)
		rtn := make(map[string]int)
		for _, view := range views {
			if view == "" {
				continue
			}
			if !viewRe.MatchString(view) {
				continue
			}
			rtn[view]++
		}
		return rtn, nil
	})
}

type idDataType struct {
	OId     string
	Version int
	Data    []byte
}

func genericCastWithErr[T any](v any, err error) (T, error) {
	if err != nil {
		var zeroVal T
		return zeroVal, err
	}
	if v == nil {
		var zeroVal T
		return zeroVal, nil
	}
	return v.(T), err
}

func DBGetSingleton[T starobj.StarObj](ctx context.Context) (T, error) {
	rtn, err := DBGetSingletonByType(ctx, getOTypeGen[T]())
	return genericCastWithErr[T](rtn, err)
}

func DBGetSingletonByType(ctx context.Context, otype string) (starobj.StarObj, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) (starobj.StarObj, error) {
		table := tableNameFromOType(otype)
		query := fmt.Sprintf("SELECT oid, version, data FROM %s LIMIT 1", table)
		var row idDataType
		found := tx.Get(&row, query)
		if !found {
			return nil, ErrNotFound
		}
		rtn, err := starobj.FromJson(row.Data)
		if err != nil {
			return rtn, err
		}
		starobj.SetVersion(rtn, row.Version)
		return rtn, nil
	})
}

func DBExistsORef(ctx context.Context, oref starobj.ORef) (bool, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) (bool, error) {
		table := tableNameFromOType(oref.OType)
		query := fmt.Sprintf("SELECT oid FROM %s WHERE oid = ?", table)
		return tx.Exists(query, oref.OID), nil
	})
}

func DBGet[T starobj.StarObj](ctx context.Context, id string) (T, error) {
	rtn, err := DBGetORef(ctx, starobj.ORef{OType: getOTypeGen[T](), OID: id})
	return genericCastWithErr[T](rtn, err)
}

func DBMustGet[T starobj.StarObj](ctx context.Context, id string) (T, error) {
	rtn, err := DBGetORef(ctx, starobj.ORef{OType: getOTypeGen[T](), OID: id})
	if err != nil {
		var zeroVal T
		return zeroVal, err
	}
	if rtn == nil {
		var zeroVal T
		return zeroVal, ErrNotFound
	}
	return rtn.(T), nil
}

func DBGetORef(ctx context.Context, oref starobj.ORef) (starobj.StarObj, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) (starobj.StarObj, error) {
		table := tableNameFromOType(oref.OType)
		query := fmt.Sprintf("SELECT oid, version, data FROM %s WHERE oid = ?", table)
		var row idDataType
		found := tx.Get(&row, query, oref.OID)
		if !found {
			return nil, nil
		}
		rtn, err := starobj.FromJson(row.Data)
		if err != nil {
			return rtn, err
		}
		starobj.SetVersion(rtn, row.Version)
		return rtn, nil
	})
}

func dbSelectOIDs(ctx context.Context, otype string, oids []string) ([]starobj.StarObj, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) ([]starobj.StarObj, error) {
		table := tableNameFromOType(otype)
		query := fmt.Sprintf("SELECT oid, version, data FROM %s WHERE oid IN (SELECT value FROM json_each(?))", table)
		var rows []idDataType
		tx.Select(&rows, query, dbutil.QuickJson(oids))
		rtn := make([]starobj.StarObj, 0, len(rows))
		for _, row := range rows {
			starObj, err := starobj.FromJson(row.Data)
			if err != nil {
				return nil, err
			}
			starobj.SetVersion(starObj, row.Version)
			rtn = append(rtn, starObj)
		}
		return rtn, nil
	})
}

func DBSelectORefs(ctx context.Context, orefs []starobj.ORef) ([]starobj.StarObj, error) {
	oidsByType := make(map[string][]string)
	for _, oref := range orefs {
		oidsByType[oref.OType] = append(oidsByType[oref.OType], oref.OID)
	}
	return WithTxRtn(ctx, func(tx *TxWrap) ([]starobj.StarObj, error) {
		rtn := make([]starobj.StarObj, 0, len(orefs))
		for otype, oids := range oidsByType {
			rtnArr, err := dbSelectOIDs(tx.Context(), otype, oids)
			if err != nil {
				return nil, err
			}
			rtn = append(rtn, rtnArr...)
		}
		return rtn, nil
	})
}

func DBGetAllOIDsByType(ctx context.Context, otype string) ([]string, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) ([]string, error) {
		rtn := make([]string, 0)
		table := tableNameFromOType(otype)
		log.Printf("DBGetAllOIDsByType table: %s\n", table)
		query := fmt.Sprintf("SELECT oid FROM %s", table)
		var rows []idDataType
		tx.Select(&rows, query)
		for _, row := range rows {
			rtn = append(rtn, row.OId)
		}
		return rtn, nil
	})
}

func DBGetAllObjsByType[T starobj.StarObj](ctx context.Context, otype string) ([]T, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) ([]T, error) {
		rtn := make([]T, 0)
		table := tableNameFromOType(otype)
		log.Printf("DBGetAllObjsByType table: %s\n", table)
		query := fmt.Sprintf("SELECT oid, version, data FROM %s", table)
		var rows []idDataType
		tx.Select(&rows, query)
		for _, row := range rows {
			starObj, err := starobj.FromJson(row.Data)
			if err != nil {
				return nil, err
			}
			starobj.SetVersion(starObj, row.Version)

			rtn = append(rtn, starObj.(T))
		}
		return rtn, nil
	})
}

func DBResolveEasyOID(ctx context.Context, oid string) (*starobj.ORef, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) (*starobj.ORef, error) {
		for _, rtype := range starobj.AllStarObjTypes() {
			otype := reflect.Zero(rtype).Interface().(starobj.StarObj).GetOType()
			table := tableNameFromOType(otype)
			var fullOID string
			if len(oid) == 8 {
				query := fmt.Sprintf("SELECT oid FROM %s WHERE oid LIKE ?", table)
				fullOID = tx.GetString(query, oid+"%")
			} else {
				query := fmt.Sprintf("SELECT oid FROM %s WHERE oid = ?", table)
				fullOID = tx.GetString(query, oid)
			}
			if fullOID != "" {
				oref := starobj.MakeORef(otype, fullOID)
				return &oref, nil
			}
		}
		return nil, ErrNotFound
	})
}

func DBSelectMap[T starobj.StarObj](ctx context.Context, ids []string) (map[string]T, error) {
	rtnArr, err := dbSelectOIDs(ctx, getOTypeGen[T](), ids)
	if err != nil {
		return nil, err
	}
	rtnMap := make(map[string]T)
	for _, obj := range rtnArr {
		rtnMap[starobj.GetOID(obj)] = obj.(T)
	}
	return rtnMap, nil
}

func DBDelete(ctx context.Context, otype string, id string) error {
	err := WithTx(ctx, func(tx *TxWrap) error {
		table := tableNameFromOType(otype)
		query := fmt.Sprintf("DELETE FROM %s WHERE oid = ?", table)
		tx.Exec(query, id)
		starobj.ContextAddUpdate(ctx, starobj.StarObjUpdate{UpdateType: starobj.UpdateType_Delete, OType: otype, OID: id})
		return nil
	})
	if err != nil {
		return err
	}
	go func() {
		defer func() {
			panichandler.PanicHandler("DBDelete:filestore.DeleteZone", recover())
		}()
		// we spawn a go routine here because we don't want to reuse the DB connection
		// since DBDelete is called in a transaction from DeleteTab
		deleteCtx, cancelFn := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelFn()
		err := filestore.WFS.DeleteZone(deleteCtx, id)
		if err != nil {
			log.Printf("error deleting filestore zone (after deleting block): %v", err)
		}
	}()
	return nil
}

func DBUpdate(ctx context.Context, val starobj.StarObj) error {
	oid := starobj.GetOID(val)
	if oid == "" {
		return fmt.Errorf("cannot update %T value with empty id", val)
	}
	jsonData, err := starobj.ToJson(val)
	if err != nil {
		return err
	}
	return WithTx(ctx, func(tx *TxWrap) error {
		table := starObjTableName(val)
		query := fmt.Sprintf("UPDATE %s SET data = ?, version = version+1 WHERE oid = ? RETURNING version", table)
		newVersion := tx.GetInt(query, jsonData, oid)
		starobj.SetVersion(val, newVersion)
		starobj.ContextAddUpdate(ctx, starobj.StarObjUpdate{UpdateType: starobj.UpdateType_Update, OType: val.GetOType(), OID: oid, Obj: val})
		return nil
	})
}

func DBInsert(ctx context.Context, val starobj.StarObj) error {
	oid := starobj.GetOID(val)
	if oid == "" {
		return fmt.Errorf("cannot insert %T value with empty id", val)
	}
	jsonData, err := starobj.ToJson(val)
	if err != nil {
		return err
	}
	return WithTx(ctx, func(tx *TxWrap) error {
		table := starObjTableName(val)
		starobj.SetVersion(val, 1)
		query := fmt.Sprintf("INSERT INTO %s (oid, version, data) VALUES (?, ?, ?)", table)
		tx.Exec(query, oid, 1, jsonData)
		starobj.ContextAddUpdate(ctx, starobj.StarObjUpdate{UpdateType: starobj.UpdateType_Update, OType: val.GetOType(), OID: oid, Obj: val})
		return nil
	})
}

func DBFindTabForBlockId(ctx context.Context, blockId string) (string, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) (string, error) {
		iterNum := 1
		for {
			if iterNum > 5 {
				return "", fmt.Errorf("too many iterations looking for tab in block parents")
			}
			query := `
			SELECT json_extract(b.data, '$.parentoref') AS parentoref
			FROM db_block b
			WHERE b.oid = ?;`
			parentORef := tx.GetString(query, blockId)
			oref, err := starobj.ParseORef(parentORef)
			if err != nil {
				return "", fmt.Errorf("bad block parent oref: %v", err)
			}
			if oref.OType == "tab" {
				return oref.OID, nil
			}
			if oref.OType == "block" {
				blockId = oref.OID
				iterNum++
				continue
			}
			return "", fmt.Errorf("bad parent oref type: %v", oref.OType)
		}
	})
}

func DBFindWorkspaceForTabId(ctx context.Context, tabId string) (string, error) {
	log.Printf("DBFindWorkspaceForTabId tabId: %s\n", tabId)
	return WithTxRtn(ctx, func(tx *TxWrap) (string, error) {
		query := `
			WITH variable(value) AS (
				SELECT ?
			)
			SELECT w.oid
			FROM db_workspace w, variable
			WHERE EXISTS (
				SELECT 1
				FROM json_each(w.data, '$.tabids') AS je
				WHERE je.value = variable.value
			)
			OR EXISTS (
				SELECT 1
				FROM json_each(w.data, '$.pinnedtabids') AS je
				WHERE je.value = variable.value
			);
			`
		wsId := tx.GetString(query, tabId)
		log.Printf("DBFindWorkspaceForTabId wsId: %s\n", wsId)
		return wsId, nil
	})
}

func DBFindWindowForWorkspaceId(ctx context.Context, workspaceId string) (string, error) {
	return WithTxRtn(ctx, func(tx *TxWrap) (string, error) {
		query := `
			SELECT w.oid
			FROM db_window w WHERE json_extract(data, '$.workspaceid') = ?`
		return tx.GetString(query, workspaceId), nil
	})
}
