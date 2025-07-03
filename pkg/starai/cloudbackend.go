// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package starai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/commandlinedev/starterm/pkg/panichandler"
	"github.com/commandlinedev/starterm/pkg/wcloud"
	"github.com/commandlinedev/starterm/pkg/wshrpc"
	"github.com/gorilla/websocket"
)

type StarAICloudBackend struct{}

var _ AIBackend = StarAICloudBackend{}

const CloudWebsocketConnectTimeout = 1 * time.Minute
const OpenAICloudReqStr = "openai-cloudreq"
const PacketEOFStr = "EOF"

type StarAICloudReqPacketType struct {
	Type       string                           `json:"type"`
	ClientId   string                           `json:"clientid"`
	Prompt     []wshrpc.StarAIPromptMessageType `json:"prompt"`
	MaxTokens  int                              `json:"maxtokens,omitempty"`
	MaxChoices int                              `json:"maxchoices,omitempty"`
}

func MakeStarAICloudReqPacket() *StarAICloudReqPacketType {
	return &StarAICloudReqPacketType{
		Type: OpenAICloudReqStr,
	}
}

func (StarAICloudBackend) StreamCompletion(ctx context.Context, request wshrpc.StarAIStreamRequest) chan wshrpc.RespOrErrorUnion[wshrpc.StarAIPacketType] {
	rtn := make(chan wshrpc.RespOrErrorUnion[wshrpc.StarAIPacketType])
	wsEndpoint := wcloud.GetWSEndpoint()
	go func() {
		defer func() {
			panicErr := panichandler.PanicHandler("StarAICloudBackend.StreamCompletion", recover())
			if panicErr != nil {
				rtn <- makeAIError(panicErr)
			}
			close(rtn)
		}()
		if wsEndpoint == "" {
			rtn <- makeAIError(fmt.Errorf("no cloud ws endpoint found"))
			return
		}
		if request.Opts == nil {
			rtn <- makeAIError(fmt.Errorf("no openai opts found"))
			return
		}
		websocketContext, dialCancelFn := context.WithTimeout(context.Background(), CloudWebsocketConnectTimeout)
		defer dialCancelFn()
		conn, _, err := websocket.DefaultDialer.DialContext(websocketContext, wsEndpoint, nil)
		if err == context.DeadlineExceeded {
			rtn <- makeAIError(fmt.Errorf("OpenAI request, timed out connecting to cloud server: %v", err))
			return
		} else if err != nil {
			rtn <- makeAIError(fmt.Errorf("OpenAI request, websocket connect error: %v", err))
			return
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				rtn <- makeAIError(fmt.Errorf("unable to close openai channel: %v", err))
			}
		}()
		var sendablePromptMsgs []wshrpc.StarAIPromptMessageType
		for _, promptMsg := range request.Prompt {
			if promptMsg.Role == "error" {
				continue
			}
			sendablePromptMsgs = append(sendablePromptMsgs, promptMsg)
		}
		reqPk := MakeStarAICloudReqPacket()
		reqPk.ClientId = request.ClientId
		reqPk.Prompt = sendablePromptMsgs
		reqPk.MaxTokens = request.Opts.MaxTokens
		reqPk.MaxChoices = request.Opts.MaxChoices
		configMessageBuf, err := json.Marshal(reqPk)
		if err != nil {
			rtn <- makeAIError(fmt.Errorf("OpenAI request, packet marshal error: %v", err))
			return
		}
		err = conn.WriteMessage(websocket.TextMessage, configMessageBuf)
		if err != nil {
			rtn <- makeAIError(fmt.Errorf("OpenAI request, websocket write config error: %v", err))
			return
		}
		for {
			_, socketMessage, err := conn.ReadMessage()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("err received: %v", err)
				rtn <- makeAIError(fmt.Errorf("OpenAI request, websocket error reading message: %v", err))
				break
			}
			var streamResp *wshrpc.StarAIPacketType
			err = json.Unmarshal(socketMessage, &streamResp)
			if err != nil {
				rtn <- makeAIError(fmt.Errorf("OpenAI request, websocket response json decode error: %v", err))
				break
			}
			if streamResp.Error == PacketEOFStr {
				// got eof packet from socket
				break
			} else if streamResp.Error != "" {
				// use error from server directly
				rtn <- makeAIError(fmt.Errorf("%v", streamResp.Error))
				break
			}
			rtn <- wshrpc.RespOrErrorUnion[wshrpc.StarAIPacketType]{Response: *streamResp}
		}
	}()
	return rtn
}
