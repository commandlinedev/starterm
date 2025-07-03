// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package score

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/commandlinedev/starterm/pkg/blockcontroller"
	"github.com/commandlinedev/starterm/pkg/filestore"
	"github.com/commandlinedev/starterm/pkg/panichandler"
	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/telemetry"
	"github.com/commandlinedev/starterm/pkg/telemetry/telemetrydata"
	"github.com/commandlinedev/starterm/pkg/util/utilfn"
	"github.com/commandlinedev/starterm/pkg/wps"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wstore"
	"github.com/google/uuid"
)

func CreateSubBlock(ctx context.Context, blockId string, blockDef *starobj.BlockDef) (*starobj.Block, error) {
	if blockDef == nil {
		return nil, fmt.Errorf("blockDef is nil")
	}
	if blockDef.Meta == nil || blockDef.Meta.GetString(starobj.MetaKey_View, "") == "" {
		return nil, fmt.Errorf("no view provided for new block")
	}
	blockData, err := createSubBlockObj(ctx, blockId, blockDef)
	if err != nil {
		return nil, fmt.Errorf("error creating sub block: %w", err)
	}
	return blockData, nil
}

func createSubBlockObj(ctx context.Context, parentBlockId string, blockDef *starobj.BlockDef) (*starobj.Block, error) {
	return wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (*starobj.Block, error) {
		parentBlock, _ := wstore.DBGet[*starobj.Block](tx.Context(), parentBlockId)
		if parentBlock == nil {
			return nil, fmt.Errorf("parent block not found: %q", parentBlockId)
		}
		blockId := uuid.NewString()
		blockData := &starobj.Block{
			OID:         blockId,
			ParentORef:  starobj.MakeORef(starobj.OType_Block, parentBlockId).String(),
			RuntimeOpts: nil,
			Meta:        blockDef.Meta,
		}
		wstore.DBInsert(tx.Context(), blockData)
		parentBlock.SubBlockIds = append(parentBlock.SubBlockIds, blockId)
		wstore.DBUpdate(tx.Context(), parentBlock)
		return blockData, nil
	})
}

func CreateBlock(ctx context.Context, tabId string, blockDef *starobj.BlockDef, rtOpts *starobj.RuntimeOpts) (rtnBlock *starobj.Block, rtnErr error) {
	var blockCreated bool
	var newBlockOID string
	defer func() {
		if rtnErr == nil {
			return
		}
		// if there was an error, and we created the block, clean it up since the function failed
		if blockCreated && newBlockOID != "" {
			deleteBlockObj(ctx, newBlockOID)
			filestore.WFS.DeleteZone(ctx, newBlockOID)
		}
	}()
	if blockDef == nil {
		return nil, fmt.Errorf("blockDef is nil")
	}
	if blockDef.Meta == nil || blockDef.Meta.GetString(starobj.MetaKey_View, "") == "" {
		return nil, fmt.Errorf("no view provided for new block")
	}
	blockData, err := createBlockObj(ctx, tabId, blockDef, rtOpts)
	if err != nil {
		return nil, fmt.Errorf("error creating block: %w", err)
	}
	blockCreated = true
	newBlockOID = blockData.OID
	// upload the files if present
	if len(blockDef.Files) > 0 {
		for fileName, fileDef := range blockDef.Files {
			err := filestore.WFS.MakeFile(ctx, newBlockOID, fileName, fileDef.Meta, wshrpc.FileOpts{})
			if err != nil {
				return nil, fmt.Errorf("error making blockfile %q: %w", fileName, err)
			}
			err = filestore.WFS.WriteFile(ctx, newBlockOID, fileName, []byte(fileDef.Content))
			if err != nil {
				return nil, fmt.Errorf("error writing blockfile %q: %w", fileName, err)
			}
		}
	}
	go func() {
		defer func() {
			panichandler.PanicHandler("CreateBlock:telemetry", recover())
		}()
		blockView := blockDef.Meta.GetString(starobj.MetaKey_View, "")
		if blockView == "" {
			return
		}
		tctx, cancelFn := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelFn()
		telemetry.UpdateActivity(tctx, wshrpc.ActivityUpdate{
			Renderers: map[string]int{blockView: 1},
		})
		telemetry.RecordTEvent(tctx, &telemetrydata.TEvent{
			Event: "action:createblock",
			Props: telemetrydata.TEventProps{
				BlockView: blockView,
			},
		})
	}()
	return blockData, nil
}

func createBlockObj(ctx context.Context, tabId string, blockDef *starobj.BlockDef, rtOpts *starobj.RuntimeOpts) (*starobj.Block, error) {
	return wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (*starobj.Block, error) {
		tab, _ := wstore.DBGet[*starobj.Tab](tx.Context(), tabId)
		if tab == nil {
			return nil, fmt.Errorf("tab not found: %q", tabId)
		}
		blockId := uuid.NewString()
		blockData := &starobj.Block{
			OID:         blockId,
			ParentORef:  starobj.MakeORef(starobj.OType_Tab, tabId).String(),
			RuntimeOpts: rtOpts,
			Meta:        blockDef.Meta,
		}
		wstore.DBInsert(tx.Context(), blockData)
		tab.BlockIds = append(tab.BlockIds, blockId)
		wstore.DBUpdate(tx.Context(), tab)
		return blockData, nil
	})
}

// Must delete all blocks individually first.
// Also deletes LayoutState.
// recursive: if true, will recursively close parent tab, window, workspace, if they are empty.
// Returns new active tab id, error.
func DeleteBlock(ctx context.Context, blockId string, recursive bool) error {
	block, err := wstore.DBMustGet[*starobj.Block](ctx, blockId)
	if err != nil {
		return fmt.Errorf("error getting block: %w", err)
	}
	if block == nil {
		return nil
	}
	if len(block.SubBlockIds) > 0 {
		for _, subBlockId := range block.SubBlockIds {
			err := DeleteBlock(ctx, subBlockId, recursive)
			if err != nil {
				return fmt.Errorf("error deleting subblock %s: %w", subBlockId, err)
			}
		}
	}
	parentBlockCount, err := deleteBlockObj(ctx, blockId)
	if err != nil {
		return fmt.Errorf("error deleting block: %w", err)
	}
	log.Printf("DeleteBlock: parentBlockCount: %d", parentBlockCount)
	parentORef := starobj.ParseORefNoErr(block.ParentORef)

	if recursive && parentORef.OType == starobj.OType_Tab && parentBlockCount == 0 {
		// if parent tab has no blocks, delete the tab
		log.Printf("DeleteBlock: parent tab has no blocks, deleting tab %s", parentORef.OID)
		parentWorkspaceId, err := wstore.DBFindWorkspaceForTabId(ctx, parentORef.OID)
		if err != nil {
			return fmt.Errorf("error finding workspace for tab to delete %s: %w", parentORef.OID, err)
		}
		newActiveTabId, err := DeleteTab(ctx, parentWorkspaceId, parentORef.OID, true)
		if err != nil {
			return fmt.Errorf("error deleting tab %s: %w", parentORef.OID, err)
		}
		SendActiveTabUpdate(ctx, parentWorkspaceId, newActiveTabId)
	}
	go blockcontroller.StopBlockController(blockId)
	sendBlockCloseEvent(blockId)
	return nil
}

// returns the updated block count for the parent object
func deleteBlockObj(ctx context.Context, blockId string) (int, error) {
	return wstore.WithTxRtn(ctx, func(tx *wstore.TxWrap) (int, error) {
		block, err := wstore.DBGet[*starobj.Block](tx.Context(), blockId)
		if err != nil {
			return -1, fmt.Errorf("error getting block: %w", err)
		}
		if block == nil {
			return -1, fmt.Errorf("block not found: %q", blockId)
		}
		if len(block.SubBlockIds) > 0 {
			return -1, fmt.Errorf("block has subblocks, must delete subblocks first")
		}
		parentORef := starobj.ParseORefNoErr(block.ParentORef)
		parentBlockCount := -1
		if parentORef != nil {
			if parentORef.OType == starobj.OType_Tab {
				tab, _ := wstore.DBGet[*starobj.Tab](tx.Context(), parentORef.OID)
				if tab != nil {
					tab.BlockIds = utilfn.RemoveElemFromSlice(tab.BlockIds, blockId)
					wstore.DBUpdate(tx.Context(), tab)
					parentBlockCount = len(tab.BlockIds)
				}
			} else if parentORef.OType == starobj.OType_Block {
				parentBlock, _ := wstore.DBGet[*starobj.Block](tx.Context(), parentORef.OID)
				if parentBlock != nil {
					parentBlock.SubBlockIds = utilfn.RemoveElemFromSlice(parentBlock.SubBlockIds, blockId)
					wstore.DBUpdate(tx.Context(), parentBlock)
					parentBlockCount = len(parentBlock.SubBlockIds)
				}
			}
		}
		wstore.DBDelete(tx.Context(), starobj.OType_Block, blockId)
		return parentBlockCount, nil
	})
}

func sendBlockCloseEvent(blockId string) {
	starEvent := wps.StarEvent{
		Event: wps.Event_BlockClose,
		Scopes: []string{
			starobj.MakeORef(starobj.OType_Block, blockId).String(),
		},
		Data: blockId,
	}
	wps.Broker.Publish(starEvent)
}
