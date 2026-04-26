package feishu

import (
	"context"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestApplyUpdateCardFallsBackToSendWhenTargetIsText(t *testing.T) {
	gateway := NewLiveGateway(LiveGatewayConfig{GatewayID: "app-1"})
	gateway.patchMessageFn = func(ctx context.Context, messageID, content string) (*larkim.PatchMessageResp, error) {
		return &larkim.PatchMessageResp{
			ApiResp: &larkcore.ApiResp{},
			CodeError: larkcore.CodeError{
				Code: 230001,
				Msg:  "Your request contains an invalid request parameter, ext=This message is NOT a card.",
			},
		}, nil
	}
	var createdMsgType string
	var createdReceiveID string
	gateway.createMessageFn = func(ctx context.Context, receiveIDType, receiveID, msgType, content string) (*larkim.CreateMessageResp, error) {
		createdMsgType = msgType
		createdReceiveID = receiveID
		messageID := "om-card-new"
		return &larkim.CreateMessageResp{
			ApiResp: &larkcore.ApiResp{},
			CodeError: larkcore.CodeError{
				Code: 0,
				Msg:  "ok",
			},
			Data: &larkim.CreateMessageRespData{
				MessageId: stringRef(messageID),
			},
		}, nil
	}

	err := gateway.Apply(t.Context(), []Operation{{
		Kind:      OperationUpdateCard,
		GatewayID: "app-1",
		ChatID:    "oc-chat-1",
		MessageID: "om-text-1",
		CardTitle: "最后答复",
		CardBody:  "正文",
		card:      rawCardDocument("最后答复", "正文", cardThemeFinal, nil),
	}})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if createdMsgType != "interactive" || createdReceiveID != "oc-chat-1" {
		t.Fatalf("expected fallback card create, msgType=%q receiveID=%q", createdMsgType, createdReceiveID)
	}
}
