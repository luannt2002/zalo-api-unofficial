# Bản khởi tạo SDK — 08/10/2026

Repo chính: luannt2002/zalo-api-unofficial. Source từ SDK Go bản tách đã kiểm trước đó, imports đổi module path; không sửa hành vi protocol. Repo mới khởi tạo một root commit chứa code/tests/license/skill/docs; không rewrite hoặc xoá backup repo local cũ.

Provenance source Go362cdb96b5d33e2147b80629d702baa8fb8de378. Baseline JS lịch sử54df45d803fad7397eca04b2753cc0a894dc6e86 được xác nhận tồn tại qua GitHub API; observed upstream HEADdadfef18bcac53537741855c99f084152da1fad9. Inventory nguồn là evidence; file baseline được referenced hiện vắng. Không claim complete parity. UPSTREAM.json ghi rõ checked/verified distinction.

Tag khởi tạo dự kiến v0.1.0 sau verify/review. Không login account thật, không copy credentials hoặc private keys. Package skill ở skills/zca-go-maintainer, portable .skill ở /home/luannt/application/zca-go-maintainer.skill. Skill không thay source SDK; đã validate frontmatter/resources.

Kiểm chứng bản tên mới: build/vet toàn module và race suite ./... đều exit0; skill validator hợp lệ, UPSTREAM JSON và links mới hợp lệ; review read-only KET-LUAN: DAT. Tag v0.1.0 đánh version SDK, không chứng nhận vendor thật/parity.
