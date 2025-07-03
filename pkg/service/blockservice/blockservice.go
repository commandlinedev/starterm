// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package blockservice

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/commandlinedev/starterm/pkg/blockcontroller"
	"github.com/commandlinedev/starterm/pkg/filestore"
	"github.com/commandlinedev/starterm/pkg/starobj"
	"github.com/commandlinedev/starterm/pkg/tsgen/tsgenmeta"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/commandlinedev/starterm/pkg/wstore"
)

type BlockService struct{}

const DefaultTimeout = 2 * time.Second

var BlockServiceInstance = &BlockService{}

func (bs *BlockService) SendCommand_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{
		Desc:     "send command to block",
		ArgNames: []string{"blockid", "cmd"},
	}
}

func (bs *BlockService) GetControllerStatus(ctx context.Context, blockId string) (*blockcontroller.BlockControllerRuntimeStatus, error) {
	bc := blockcontroller.GetBlockController(blockId)
	if bc == nil {
		return nil, nil
	}
	return bc.GetRuntimeStatus(), nil
}

func (*BlockService) SaveTerminalState_Meta() tsgenmeta.MethodMeta {
	return tsgenmeta.MethodMeta{
		Desc:     "save the terminal state to a blockfile",
		ArgNames: []string{"ctx", "blockId", "state", "stateType", "ptyOffset", "termSize"},
	}
}

func (bs *BlockService) SaveTerminalState(ctx context.Context, blockId string, state string, stateType string, ptyOffset int64, termSize starobj.TermSize) error {
	_, err := wstore.DBMustGet[*starobj.Block](ctx, blockId)
	if err != nil {
		return err
	}
	if stateType != "full" && stateType != "preview" {
		return fmt.Errorf("invalid state type: %q", stateType)
	}
	// ignore MakeFile error (already exists is ok)
	filestore.WFS.MakeFile(ctx, blockId, "cache:term:"+stateType, nil, wshrpc.FileOpts{})
	err = filestore.WFS.WriteFile(ctx, blockId, "cache:term:"+stateType, []byte(state))
	if err != nil {
		return fmt.Errorf("cannot save terminal state: %w", err)
	}
	fileMeta := wshrpc.FileMeta{
		"ptyoffset": ptyOffset,
		"termsize":  termSize,
	}
	err = filestore.WFS.WriteMeta(ctx, blockId, "cache:term:"+stateType, fileMeta, true)
	if err != nil {
		return fmt.Errorf("cannot save terminal state meta: %w", err)
	}
	return nil
}

func (bs *BlockService) SaveStarAiData(ctx context.Context, blockId string, history []wshrpc.StarAIPromptMessageType) error {
	block, err := wstore.DBMustGet[*starobj.Block](ctx, blockId)
	if err != nil {
		return err
	}
	viewName := block.Meta.GetString(starobj.MetaKey_View, "")
	if viewName != "starai" {
		return fmt.Errorf("invalid view type: %s", viewName)
	}
	historyBytes, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("unable to serialize ai history: %v", err)
	}
	// ignore MakeFile error (already exists is ok)
	filestore.WFS.MakeFile(ctx, blockId, "aidata", nil, wshrpc.FileOpts{})
	err = filestore.WFS.WriteFile(ctx, blockId, "aidata", historyBytes)
	if err != nil {
		return fmt.Errorf("cannot save terminal state: %w", err)
	}
	return nil
}
