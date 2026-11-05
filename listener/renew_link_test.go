package listener

import (
	"context"
	"testing"
	"time"
)

func TestRenewLinkEventReceived(t *testing.T) {
	ln := newRouterTestListener(t)

	// Giả lập server push WebSocket cmd 601: act_type "photo", act "renew_link"
	payload := `{"act_type":"photo","act":"renew_link","data":[{"img":"orig.jpg","originalUrl":"http://orig","normal":"http://normal.jpg","hd":"http://hd.jpg","thumb":"http://thumb.jpg","destId":"123456","destType":1,"errorCode":0,"expiredTime":1762319000}]}`
	ln.router(context.Background(), 1, 601, 0, controlFrame(payload))

	select {
	case ev := <-ln.RenewLink():
		if ev.ActionType != "photo" || ev.Action != "renew_link" {
			t.Fatalf("unexpected act_type/act: %s/%s", ev.ActionType, ev.Action)
		}
		if len(ev.Items) != 1 {
			t.Fatalf("expected 1 item, got %d", len(ev.Items))
		}
		item := ev.Items[0]
		if item.Normal != "http://normal.jpg" || item.HD != "http://hd.jpg" || item.DestID != "123456" {
			t.Fatalf("unexpected item fields: %+v", item)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for RenewLink event")
	}
}

func TestPhotoSuccessEventReceived(t *testing.T) {
	ln := newRouterTestListener(t)

	payload := `{"act_type":"photo_success","act":"success","fileId":987654321,"data":{"url":"http://photo/1"}}`
	ln.router(context.Background(), 1, 601, 0, controlFrame(payload))

	select {
	case ev := <-ln.PhotoSuccess():
		if ev.ActionType != "photo_success" {
			t.Fatalf("unexpected act_type: %s", ev.ActionType)
		}
		if ev.FileID != 987654321 {
			t.Fatalf("expected fileId 987654321, got %d", ev.FileID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for PhotoSuccess event")
	}
}

func TestResetCloudEventReceived(t *testing.T) {
	ln := newRouterTestListener(t)

	// Giả lập cmd 620
	ln.router(context.Background(), 1, 620, 0, plainFrame(`{"status":"cloud_reset"}`))

	select {
	case ev := <-ln.ResetCloud():
		if ev.CMD != 620 {
			t.Fatalf("expected CMD 620, got %d", ev.CMD)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ResetCloud event")
	}
}

