// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package objectservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/tsgen/tsgenmeta"
	"github.com/commandlinedev/starterm/pkg/wcore"
	"github.com/commandlinedev/starterm/pkg/wps"
	"github.com/commandlinedev/starterm/pkg/wstore"
)

type ObjectService struct{}

const DefaultTimeout = 2 * time.Second
const ConnContextTimeout = 60 * time.Second

func parseORef(oref string) (*starobj.ORef, error) {
	fields := strings.Split(oref, ":")
	if len(fields) != 2 {
		return nil, fmt.Errorf("invalid object reference: %q", oref)
	}
	return &starobj.ORef{OType: fields[0], OID: fields[1]}, nil
}

func (svc *ObjectService) GetObject_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{
		Desc:     "get star object by oref",
		ArgNames: []string{"oref"},
	}
}

func (svc *ObjectService) GetObject(orefStr string) (starobj.StarObj, error) {
	oref, err := parseORef(orefStr)
	if err != nil {
		return nil, err
	}
	ctx, cancelFn := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancelFn()
	obj, err := wstore.DBGetORef(ctx, *oref)
	if err != nil {
		return nil, fmt.Errorf("error getting object: %w", err)
	}
	return obj, nil
}

func (svc *ObjectService) GetObjects_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{
		ArgNames:   []string{"orefs"},
		ReturnDesc: "objects",
	}
}

func (svc *ObjectService) GetObjects(orefStrArr []string) ([]starobj.StarObj, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancelFn()

	var orefArr []starobj.ORef
	for _, orefStr := range orefStrArr {
		orefObj, err := parseORef(orefStr)
		if err != nil {
			return nil, err
		}
		orefArr = append(orefArr, *orefObj)
	}
	return wstore.DBSelectORefs(ctx, orefArr)
}

func (svc *ObjectService) UpdateTabName_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{
		ArgNames: []string{"uiContext", "tabId", "name"},
	}
}

func (svc *ObjectService) UpdateTabName(uiContext starobj.UIContext, tabId, name string) (starobj.UpdatesRtnType, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancelFn()
	ctx = starobj.ContextWithUpdates(ctx)
	err := wstore.UpdateTabName(ctx, tabId, name)
	if err != nil {
		return nil, fmt.Errorf("error updating tab name: %w", err)
	}
	return starobj.ContextGetUpdatesRtn(ctx), nil
}

func (svc *ObjectService) CreateBlock_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{
		ArgNames:   []string{"uiContext", "blockDef", "rtOpts"},
		ReturnDesc: "blockId",
	}
}

func (svc *ObjectService) CreateBlock(uiContext starobj.UIContext, blockDef *starobj.BlockDef, rtOpts *starobj.RuntimeOpts) (string, starobj.UpdatesRtnType, error) {
	if uiContext.ActiveTabId == "" {
		return "", nil, fmt.Errorf("no active tab")
	}
	ctx, cancelFn := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancelFn()
	ctx = starobj.ContextWithUpdates(ctx)

	blockData, err := wcore.CreateBlock(ctx, uiContext.ActiveTabId, blockDef, rtOpts)
	if err != nil {
		return "", nil, err
	}

	return blockData.OID, starobj.ContextGetUpdatesRtn(ctx), nil
}

func (svc *ObjectService) DeleteBlock_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{
		ArgNames: []string{"uiContext", "blockId"},
	}
}

func (svc *ObjectService) DeleteBlock(uiContext starobj.UIContext, blockId string) (starobj.UpdatesRtnType, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancelFn()
	ctx = starobj.ContextWithUpdates(ctx)
	err := wcore.DeleteBlock(ctx, blockId, true)
	if err != nil {
		return nil, fmt.Errorf("error deleting block: %w", err)
	}
	return starobj.ContextGetUpdatesRtn(ctx), nil
}

func (svc *ObjectService) UpdateObjectMeta_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{
		ArgNames: []string{"uiContext", "oref", "meta"},
	}
}

func (svc *ObjectService) UpdateObjectMeta(uiContext starobj.UIContext, orefStr string, meta starobj.MetaMapType) (starobj.UpdatesRtnType, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancelFn()
	ctx = starobj.ContextWithUpdates(ctx)
	oref, err := parseORef(orefStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing object reference: %w", err)
	}
	err = wstore.UpdateObjectMeta(ctx, *oref, meta, false)
	if err != nil {
		return nil, fmt.Errorf("error updating %q meta: %w", orefStr, err)
	}
	return starobj.ContextGetUpdatesRtn(ctx), nil
}

func (svc *ObjectService) UpdateObject_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{
		ArgNames: []string{"uiContext", "starObj", "returnUpdates"},
	}
}

func (svc *ObjectService) UpdateObject(uiContext starobj.UIContext, starObj starobj.StarObj, returnUpdates bool) (starobj.UpdatesRtnType, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancelFn()
	ctx = starobj.ContextWithUpdates(ctx)
	if starObj == nil {
		return nil, fmt.Errorf("update wavobj is nil")
	}
	oref := starobj.ORefFromStarObj(starObj)
	found, err := wstore.DBExistsORef(ctx, *oref)
	if err != nil {
		return nil, fmt.Errorf("error getting object: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("object not found: %s", oref)
	}
	err = wstore.DBUpdate(ctx, starObj)
	if err != nil {
		return nil, fmt.Errorf("error updating object: %w", err)
	}
	if (starObj.GetOType() == starobj.OType_Workspace) && (starObj.(*starobj.Workspace).Name != "") {
		wps.Broker.Publish(wps.StarEvent{
			Event: wps.Event_WorkspaceUpdate})
	}
	if returnUpdates {
		return starobj.ContextGetUpdatesRtn(ctx), nil
	}
	return nil, nil
}
