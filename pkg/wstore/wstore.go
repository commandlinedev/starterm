// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package wstore

import (
	"context"
	"fmt"

	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/util/utilfn"
)

func init() {
	for _, rtype := range starobj.AllStarObjTypes() {
		starobj.RegisterType(rtype)
	}
}

func UpdateTabName(ctx context.Context, tabId, name string) error {
	return WithTx(ctx, func(tx *TxWrap) error {
		tab, _ := DBGet[*starobj.Tab](tx.Context(), tabId)
		if tab == nil {
			return fmt.Errorf("tab not found: %q", tabId)
		}
		if tabId != "" {
			tab.Name = name
			DBUpdate(tx.Context(), tab)
		}
		return nil
	})
}

func UpdateObjectMeta(ctx context.Context, oref starobj.ORef, meta starobj.MetaMapType, mergeSpecial bool) error {
	return WithTx(ctx, func(tx *TxWrap) error {
		if oref.IsEmpty() {
			return fmt.Errorf("empty object reference")
		}
		obj, _ := DBGetORef(tx.Context(), oref)
		if obj == nil {
			return ErrNotFound
		}
		objMeta := starobj.GetMeta(obj)
		if objMeta == nil {
			objMeta = make(map[string]any)
		}
		newMeta := starobj.MergeMeta(objMeta, meta, mergeSpecial)
		starobj.SetMeta(obj, newMeta)
		DBUpdate(tx.Context(), obj)
		return nil
	})
}

func MoveBlockToTab(ctx context.Context, currentTabId string, newTabId string, blockId string) error {
	return WithTx(ctx, func(tx *TxWrap) error {
		block, _ := DBGet[*starobj.Block](tx.Context(), blockId)
		if block == nil {
			return fmt.Errorf("block not found: %q", blockId)
		}
		currentTab, _ := DBGet[*starobj.Tab](tx.Context(), currentTabId)
		if currentTab == nil {
			return fmt.Errorf("current tab not found: %q", currentTabId)
		}
		newTab, _ := DBGet[*starobj.Tab](tx.Context(), newTabId)
		if newTab == nil {
			return fmt.Errorf("new tab not found: %q", newTabId)
		}
		blockIdx := utilfn.FindStringInSlice(currentTab.BlockIds, blockId)
		if blockIdx == -1 {
			return fmt.Errorf("block not found in current tab: %q", blockId)
		}
		currentTab.BlockIds = utilfn.RemoveElemFromSlice(currentTab.BlockIds, blockId)
		newTab.BlockIds = append(newTab.BlockIds, blockId)
		block.ParentORef = starobj.MakeORef(starobj.OType_Tab, newTabId).String()
		DBUpdate(tx.Context(), block)
		DBUpdate(tx.Context(), currentTab)
		DBUpdate(tx.Context(), newTab)
		return nil
	})
}
