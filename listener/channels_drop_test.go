package listener

import (
	"context"
	"testing"
)

// TestEmitDemCaTinBiVut — emit() drop-oldest phải đếm ĐÚNG số tin đã mất.
//
// Đây là ca đo lại chính con số đã dùng để mô tả lỗi: bơm 100 tin vào kênh buffer 2, không
// ai đọc. Kênh giữ được 2, nên 98 tin phải được ghi nhận là mất.
//
// Bản cũ chỉ đếm ở nhánh "kênh VẪN đầy sau khi vứt" — nhánh gần như không bao giờ tới, vì
// vứt xong thì luôn còn chỗ. Kết quả: mất 98 tin, DroppedEvents() báo 0. Nguy hơn không có
// metric, vì nó tạo cảm giác đang được giám sát.
//
// Đếm bằng HIỆU trước/sau, không đọc trị tuyệt đối: droppedEvents là biến gói dùng chung
// cho mọi listener và không có đường reset — đọc trị tuyệt đối là tự làm test phụ thuộc
// thứ tự chạy.
func TestEmitDemCaTinBiVut(t *testing.T) {
	const (
		buffer = 2
		gui    = 100
	)

	ch := make(chan int, buffer)
	truoc := DroppedEvents()

	for i := 0; i < gui; i++ {
		emit(context.Background(), ch, i)
	}

	nhanDuoc := len(ch)
	mat := DroppedEvents() - truoc

	if nhanDuoc != buffer {
		t.Fatalf("kênh giữ %d tin, mong %d", nhanDuoc, buffer)
	}
	if muon := int64(gui - buffer); mat != muon {
		t.Fatalf("DroppedEvents tăng %d, mong %d — bơm %d tin vào kênh buffer %d thì "+
			"phải mất đúng %d. Báo thiếu nghĩa là nhánh vứt-tin-cũ lại không đếm.",
			mat, muon, gui, buffer, muon)
	}
}

// TestEmitKhongDemKhiKhongMat — mặt còn lại của bảng chân trị: còn chỗ thì không được đếm.
// Thiếu ca này thì "đếm mọi lúc" cũng làm ca trên xanh, và metric sẽ báo THỪA.
func TestEmitKhongDemKhiKhongMat(t *testing.T) {
	ch := make(chan int, 8)
	truoc := DroppedEvents()

	for i := 0; i < 8; i++ {
		emit(context.Background(), ch, i)
	}

	if mat := DroppedEvents() - truoc; mat != 0 {
		t.Fatalf("DroppedEvents tăng %d khi kênh còn chỗ, mong 0", mat)
	}
	if len(ch) != 8 {
		t.Fatalf("kênh giữ %d tin, mong 8", len(ch))
	}
}

// TestEmitCtxHuyThiKhongDem — ctx đã hủy ⇒ emit trả về ngay, không tin nào bị vứt nên
// không được đếm. Ca này khoá ranh giới giữa "đang tắt máy" và "đang mất tin": gộp hai thứ
// đó vào một bộ đếm thì mỗi lần deploy sẽ nổ cảnh báo mất tin.
func TestEmitCtxHuyThiKhongDem(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ch := make(chan int) // không buffer: nếu không thoát theo ctx thì sẽ rơi vào drop-oldest
	truoc := DroppedEvents()

	for i := 0; i < 10; i++ {
		emit(ctx, ch, i)
	}

	if mat := DroppedEvents() - truoc; mat != 0 {
		t.Fatalf("DroppedEvents tăng %d khi ctx đã hủy, mong 0", mat)
	}
}
